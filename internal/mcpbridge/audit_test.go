package mcpbridge

import (
	"context"
	"errors"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
)

func TestStaticBearerRecordsServiceIdentityAndRejectedAuthentication(t *testing.T) {
	for _, token := range []string{"valid", "wrong", ""} {
		trace := &auditcontext.Trace{}
		request := httptest.NewRequest("POST", "/mcp", nil)
		request = request.WithContext(auditcontext.With(request.Context(), trace))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("X-Actor-ID", "forged-human")
		handler := staticBearer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), "valid")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if token == "valid" {
			if trace.ActorType != "service_client" || trace.ActorID != "mcp-service" || trace.AuthOutcome != "authenticated" {
				t.Fatalf("%+v", trace)
			}
		} else if trace.ActorID != "" || trace.AuthOutcome != "rejected" || response.Code != 401 {
			t.Fatalf("%+v status=%d", trace, response.Code)
		}
	}
}

type protocolAuditStore struct {
	events map[string]domainaudit.Event
	calls  int
	failAt int
}

func (s *protocolAuditStore) RecordAuditEvent(_ context.Context, event domainaudit.Event) error {
	s.calls++
	if s.calls == s.failAt {
		return errors.New("secret-error")
	}
	if s.events == nil {
		s.events = map[string]domainaudit.Event{}
	}
	s.events[event.ID] = event
	return nil
}
func (s *protocolAuditStore) ListAuditEvents(context.Context, domainaudit.Filter) ([]domainaudit.Event, error) {
	return nil, nil
}

func TestProtocolAuditIncludesSemanticErrorsPrincipalAndCorrelation(t *testing.T) {
	for _, scenario := range []string{"success", "tool-error", "rpc-error", "panic"} {
		t.Run(scenario, func(t *testing.T) {
			store := &protocolAuditStore{}
			bridge := &Bridge{options: Options{Audit: store, ClientID: "managed-client", Logger: slog.Default()}}
			ctx := auditcontext.With(context.Background(), &auditcontext.Trace{RequestID: "http-request", CorrelationID: "parent-correlation", ActorID: "managed-client", ActorType: "service_client", AuthOutcome: "authenticated"})
			req := &mcp.ServerRequest[*mcp.CallToolParams]{Params: &mcp.CallToolParams{Name: "secret-unknown-tool"}, Extra: &mcp.RequestExtra{TokenInfo: &auth.TokenInfo{UserID: "verified-principal"}}}
			next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
				switch scenario {
				case "tool-error":
					return &mcp.CallToolResult{IsError: true}, nil
				case "rpc-error":
					return nil, errors.New("secret-error")
				case "panic":
					panic("secret-panic")
				default:
					return &mcp.CallToolResult{}, nil
				}
			}
			func() {
				defer func() {
					if value := recover(); scenario == "panic" && value == nil {
						t.Error("panic swallowed")
					}
				}()
				_, _ = bridge.auditMiddleware(next)(ctx, "tools/call", req)
			}()
			if len(store.events) != 1 || store.calls != 2 {
				t.Fatalf("%+v", store)
			}
			for _, event := range store.events {
				expected := domainaudit.OutcomeFailed
				if scenario == "success" {
					expected = domainaudit.OutcomeSucceeded
				}
				if event.Outcome != expected || event.ActorID != "verified-principal" || event.ActorType != "principal" || event.CorrelationID != "parent-correlation" || event.Method != "tools/call" || len(event.Metadata) != 0 {
					t.Fatalf("%+v", event)
				}
			}
		})
	}
}

func TestProtocolAuditStorageFailurePreventsDispatch(t *testing.T) {
	store := &protocolAuditStore{failAt: 1}
	bridge := &Bridge{options: Options{Audit: store, Logger: slog.Default()}}
	dispatched := false
	next := func(context.Context, string, mcp.Request) (mcp.Result, error) { dispatched = true; return nil, nil }
	_, err := bridge.auditMiddleware(next)(context.Background(), "tools/call", &mcp.ServerRequest[*mcp.CallToolParams]{})
	if err == nil || dispatched || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err=%v dispatched=%t", err, dispatched)
	}
}
