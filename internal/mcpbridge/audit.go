package mcpbridge

import (
	"context"
	"errors"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (b *Bridge) auditMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (result mcp.Result, err error) {
		started := time.Now().UTC()
		id := uuid.NewString()
		trace := auditcontext.Trace{RequestID: id, CorrelationID: id, ActorID: b.options.ClientID, ActorType: "service_client", AuthOutcome: "authenticated"}
		if parent := auditcontext.From(ctx); parent != nil {
			trace = *parent
			trace.RequestID = id
		}
		if trace.ActorID == "" {
			trace.ActorID = "mcp-service"
		}
		if extra := request.GetExtra(); extra != nil && extra.TokenInfo != nil {
			trace.ActorID, trace.ActorType = extra.TokenInfo.UserID, "principal"
		}
		ctx = auditcontext.With(ctx, &trace)
		event := domainaudit.Event{ID: id, EventType: "mcp.request", RequestID: id, CorrelationID: trace.CorrelationID,
			Method: safeRPCMethod(method), Route: "mcp", Transport: "mcp", Source: "protocol",
			ActorID: trace.ActorID, ActorType: trace.ActorType, AuthOutcome: trace.AuthOutcome,
			Outcome: domainaudit.OutcomeStarted, OccurredAt: started}
		if err := b.recordProtocol(event); err != nil {
			return nil, errors.New("audit persistence unavailable")
		}
		defer func() {
			failure := recover()
			event.DurationMS = time.Since(started).Milliseconds()
			event.Outcome = domainaudit.OutcomeSucceeded
			toolFailure := false
			if tool, ok := result.(*mcp.CallToolResult); ok && tool != nil {
				toolFailure = tool.IsError
			}
			if failure != nil || err != nil || toolFailure || ctx.Err() != nil {
				event.Outcome = domainaudit.OutcomeFailed
			}
			if b.recordProtocol(event) != nil {
				b.options.Logger.Error("MCP audit completion persistence failed")
			}
			if failure != nil {
				panic(failure)
			}
		}()
		return next(ctx, method, request)
	}
}

func (b *Bridge) recordProtocol(event domainaudit.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return b.options.Audit.RecordAuditEvent(ctx, event)
}

func safeRPCMethod(method string) string {
	switch method {
	case "initialize", "ping", "tools/list", "tools/call", "resources/list", "resources/read",
		"resources/templates/list", "prompts/list", "prompts/get", "completion/complete",
		"logging/setLevel", "notifications/initialized", "notifications/cancelled":
		return method
	default:
		return "unknown"
	}
}
