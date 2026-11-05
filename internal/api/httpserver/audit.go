package httpserver

import (
	"bufio"
	"context"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/google/uuid"
)

func setAccessIdentity(r *http.Request, actor, kind, outcome string) {
	auditcontext.Identify(r.Context(), actor, kind, outcome)
}

// AuditAccess reserves a durable record before dispatch. Synchronous writes
// provide backpressure, with no lossy queue or sampling. Records are retained
// indefinitely; no automatic deletion is performed. An unfinished record means
// the connection is active, the process stopped, or final persistence failed.
func AuditAccess(next http.Handler, routes *http.ServeMux, store ports.AuditStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now().UTC()
		id := uuid.NewString()
		identity := &auditcontext.Trace{RequestID: id, CorrelationID: id, ActorType: "anonymous", AuthOutcome: "not_evaluated"}
		r = r.WithContext(auditcontext.With(r.Context(), identity))
		pattern, transport := auditRoute(routes, r)
		source := "network"
		if isLoopbackRequest(r) {
			source = "loopback"
		}
		event := domainaudit.Event{ID: id, RequestID: id, CorrelationID: id, EventType: "api.access",
			Method: r.Method, Route: pattern, Transport: transport, Source: source, Outcome: domainaudit.OutcomeStarted,
			OccurredAt: started, ActorType: "anonymous", AuthOutcome: "not_evaluated"}
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Correlation-ID", id)
		if store == nil || persistAccess(store, event) != nil {
			log.Print("audit persistence unavailable: request rejected before dispatch")
			http.Error(w, "audit persistence unavailable", http.StatusServiceUnavailable)
			return
		}
		response := &auditResponse{ResponseWriter: w}
		defer func() {
			failure := recover()
			event = completeAccess(event, response, r, identity, started, failure)
			if err := persistAccess(store, event); err != nil {
				// The started record remains durable. Never log request data or store errors.
				log.Printf("audit completion persistence failed request_id=%s", id)
			}
			if failure != nil {
				panic(failure)
			}
		}()
		next.ServeHTTP(response, r)
	})
}

func persistAccess(store ports.AuditStore, event domainaudit.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return store.RecordAuditEvent(ctx, event)
}

// Unwrap supports ResponseController; Flush and Hijack preserve SSE/WebSocket.
type auditResponse struct {
	http.ResponseWriter
	status int
}

func (w *auditResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *auditResponse) WriteHeader(status int) {
	if status >= 100 && status < 200 && status != 101 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *auditResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func (w *auditResponse) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *auditResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijacking unsupported")
	}
	conn, buffer, err := hijacker.Hijack()
	if err == nil {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, buffer, err
}

func auditRoute(routes *http.ServeMux, r *http.Request) (string, string) {
	_, pattern := routes.Handler(r)
	if pattern == "" {
		pattern = "unmatched"
	}
	// Resolve only named target identifiers from the matched template, including
	// denied requests. Never retain an arbitrary unmatched URL.
	template := strings.Fields(pattern)
	if len(template) > 0 {
		parts := strings.Split(template[len(template)-1], "/")
		values := strings.Split(r.URL.EscapedPath(), "/")
		if len(parts) == len(values) {
			for index, part := range parts {
				if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
					key := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
					if value, err := url.PathUnescape(values[index]); err == nil {
						r.SetPathValue(key, value)
					}
				}
			}
		}
	}
	transport := "http"
	if r.URL.Path == "/mcp" {
		pattern, transport = "/mcp", "mcp"
	}
	if strings.HasPrefix(r.URL.Path, "/.well-known/") {
		pattern = "oauth-discovery"
	}
	if strings.HasPrefix(r.URL.Path, "/akoflow-api/mcp") {
		pattern = "mcp-control"
	}
	return pattern, transport
}
func completeAccess(event domainaudit.Event, response *auditResponse, r *http.Request, identity *auditcontext.Trace, started time.Time, failure any) domainaudit.Event {
	event.Status = response.status
	if event.Status == 0 {
		event.Status = http.StatusOK
	}
	event.Outcome = domainaudit.OutcomeSucceeded
	if event.Status >= 400 || r.Context().Err() != nil || failure != nil {
		event.Outcome = domainaudit.OutcomeFailed
	}
	if failure != nil && response.status == 0 {
		event.Status = http.StatusInternalServerError
	}
	event.DurationMS = time.Since(started).Milliseconds()
	event.ActorID, event.ActorType, event.AuthOutcome = identity.ActorID, identity.ActorType, identity.AuthOutcome
	event.CorrelationID = identity.CorrelationID
	if (event.Status == 401 || event.Status == 403) && event.AuthOutcome == "not_evaluated" {
		event.AuthOutcome = "rejected"
	}
	if strings.Contains(response.Header().Get("Content-Type"), "text/event-stream") {
		event.Transport = "sse"
	}
	if response.status == 101 {
		event.Transport = "websocket"
	}

	event.EnvironmentID = r.PathValue("environmentId")
	event.ResourceID = r.PathValue("resourceId")
	event.ConnectionID = r.PathValue("connectionId")
	event.RuntimeID = r.PathValue("runtimeId")
	event.SessionID = r.PathValue("sessionId")
	event.ExecutionID = r.PathValue("runId")
	event.ActivityID = r.PathValue("activityId")
	event.OperationID = r.PathValue("operationId")
	event.Metadata = accessTarget(r, event.Route)
	return event
}

// accessTarget retains a safe reference for route entities outside the common
// run/resource fields, such as workflow versions, artifacts and build runs.
func accessTarget(r *http.Request, pattern string) map[string]any {
	metadata := map[string]any{}
	for _, part := range strings.Split(pattern, "/") {
		if !strings.HasPrefix(part, "{") || !strings.HasSuffix(part, "}") {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		if value := r.PathValue(key); value != "" {
			metadata["entityType"] = strings.TrimSuffix(key, "Id")
			metadata["entityId"] = value
		}
	}
	return metadata
}
