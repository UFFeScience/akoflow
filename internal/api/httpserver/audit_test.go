package httpserver

import (
	"context"
	"errors"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type accessStore struct {
	events        map[string]domainaudit.Event
	calls, failAt int
}

func (s *accessStore) RecordAuditEvent(ctx context.Context, e domainaudit.Event) error {
	s.calls++
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.failAt == s.calls {
		return errors.New("secret database failure")
	}
	if s.events == nil {
		s.events = map[string]domainaudit.Event{}
	}
	s.events[e.ID] = e
	return nil
}
func (s *accessStore) ListAuditEvents(context.Context, domainaudit.Filter) ([]domainaudit.Event, error) {
	result := []domainaudit.Event{}
	for _, e := range s.events {
		result = append(result, e)
	}
	return result, nil
}

func TestAccessAuditMethodsRoutesIdentityAndSecrets(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			store := &accessStore{}
			mux := http.NewServeMux()
			mux.HandleFunc(method+" /resources/{resourceId}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202) })
			handler := AuditAccess(SecureAPI(mux, SecurityOptions{BearerToken: "secret-token"}), mux, store)
			r := httptest.NewRequest(method, "/resources/node?password=secret-query", strings.NewReader("secret-body"))
			r.Header.Set("Authorization", "Bearer secret-token")
			r.Header.Set("X-Actor-ID", "forged-human")
			r.Header.Set("X-Forwarded-For", "forged-source")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != 202 || len(store.events) != 1 || store.calls != 2 {
				t.Fatalf("status=%d events=%+v", w.Code, store.events)
			}
			for _, e := range store.events {
				if e.Method != method || e.Route != method+" /resources/{resourceId}" || e.ResourceID != "node" || e.Status != 202 || e.ActorID != "api-service" || e.ActorType != "service_client" || e.AuthOutcome != "authenticated" {
					t.Fatalf("%+v", e)
				}
				if e.RequestID == "" || e.CorrelationID != e.RequestID || e.RequestID != w.Header().Get("X-Request-ID") {
					t.Fatalf("missing correlation: %+v", e)
				}
				if e.Source != "network" || strings.Contains(e.Summary, "secret") {
					t.Fatalf("untrusted provenance: %+v", e)
				}
			}
		})
	}
}

func TestAccessAuditErrorsMissingRoutesAndPanics(t *testing.T) {
	for _, scenario := range []string{"unauthorized", "missing", "method", "failure", "panic", "cancelled", "stream"} {
		t.Run(scenario, func(t *testing.T) {
			store := &accessStore{}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /known", func(w http.ResponseWriter, r *http.Request) {
				switch scenario {
				case "failure":
					http.Error(w, "secret-error", 500)
				case "panic":
					panic("secret-panic")
				case "stream":
					w.Header().Set("Content-Type", "text/event-stream")
					w.(http.Flusher).Flush()
					_, _ = w.Write([]byte("secret-output"))
				default:
					w.WriteHeader(200)
				}
			})
			handler := AuditAccess(SecureAPI(mux, SecurityOptions{BearerToken: "token"}), mux, store)
			path, method := "/known", "GET"
			if scenario == "missing" {
				path = "/unknown-secret"
			}
			if scenario == "method" {
				method = "POST"
			}
			r := httptest.NewRequest(method, path, nil)
			if scenario != "unauthorized" {
				r.Header.Set("Authorization", "Bearer token")
			}
			if scenario == "cancelled" {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
			}
			func() {
				defer func() {
					value := recover()
					if scenario == "panic" && value == nil {
						t.Error("panic swallowed")
					}
					if scenario != "panic" && value != nil {
						t.Errorf("panic: %v", value)
					}
				}()
				handler.ServeHTTP(httptest.NewRecorder(), r)
			}()
			if len(store.events) != 1 {
				t.Fatalf("%+v", store.events)
			}
			for _, e := range store.events {
				expected := domainaudit.OutcomeFailed
				if scenario == "stream" {
					expected = domainaudit.OutcomeSucceeded
					if e.Transport != "sse" {
						t.Fatal(e.Transport)
					}
				}
				if e.Outcome != expected {
					t.Fatalf("%+v", e)
				}
				if scenario == "unauthorized" && (e.Status != 401 || e.AuthOutcome != "rejected" || e.ActorID != "") {
					t.Fatalf("%+v", e)
				}
				if scenario == "missing" && (e.Status != 404 || e.Route != "unmatched") {
					t.Fatalf("%+v", e)
				}
				if scenario == "method" && e.Status != 405 {
					t.Fatalf("%+v", e)
				}
				if strings.Contains(e.Route, "unknown-secret") {
					t.Fatal("raw path recorded")
				}
			}
		})
	}
}

func TestAccessAuditPersistenceFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		store := &accessStore{failAt: failAt}
		mux := http.NewServeMux()
		dispatched := false
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { dispatched = true })
		w := httptest.NewRecorder()
		AuditAccess(mux, mux, store).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if failAt == 1 {
			if dispatched || w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
				t.Fatalf("%d %t", w.Code, dispatched)
			}
		} else {
			if !dispatched || len(store.events) != 1 {
				t.Fatal("missing durable reservation")
			}
			for _, e := range store.events {
				if e.Outcome != domainaudit.OutcomeStarted {
					t.Fatalf("%+v", e)
				}
			}
		}
	}
}

func TestAccessAuditOwnReadsDoNotRecurse(t *testing.T) {
	store := &accessStore{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /audit", func(w http.ResponseWriter, r *http.Request) {
		if _, err := store.ListAuditEvents(r.Context(), domainaudit.Filter{}); err != nil {
			t.Fatal(err)
		}
	})
	for i := 0; i < 4; i++ {
		AuditAccess(mux, mux, store).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/audit", nil))
	}
	if len(store.events) != 4 || store.calls != 8 {
		t.Fatalf("events=%d writes=%d", len(store.events), store.calls)
	}
}

func TestAuditResponseControllerAndUnsupportedHijack(t *testing.T) {
	response := &auditResponse{ResponseWriter: httptest.NewRecorder()}
	if response.Unwrap() == nil {
		t.Fatal("missing unwrap")
	}
	if _, _, err := response.Hijack(); err == nil {
		t.Fatal("unsupported hijack succeeded")
	}
	response.WriteHeader(103)
	response.WriteHeader(201)
	response.WriteHeader(500)
	if response.status != 201 {
		t.Fatalf("%d", response.status)
	}
}

type streamAuditStore struct{ recorded chan domainaudit.Event }

func (s streamAuditStore) RecordAuditEvent(_ context.Context, event domainaudit.Event) error {
	s.recorded <- event
	return nil
}
func (s streamAuditStore) ListAuditEvents(context.Context, domainaudit.Filter) ([]domainaudit.Event, error) {
	return nil, nil
}

func TestAccessAuditPreservesRealWebSocketUpgrade(t *testing.T) {
	store := streamAuditStore{recorded: make(chan domainaudit.Event, 2)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /socket", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		conn.Close()
	})
	server := httptest.NewServer(AuditAccess(mux, mux, store))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/socket", nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	<-store.recorded
	select {
	case event := <-store.recorded:
		if event.Transport != "websocket" || event.Status != 101 || event.Outcome != domainaudit.OutcomeSucceeded {
			t.Fatalf("%+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upgrade completion not audited")
	}
}

func TestPublicMuxMissingRouteIsAuditedAsNotFound(t *testing.T) {
	store := &accessStore{}
	mux := NewMux(nil)
	response := httptest.NewRecorder()
	AuditAccess(mux, mux, store).ServeHTTP(response, httptest.NewRequest("GET", "/akoflow-api/unknown?token=secret", nil))
	if response.Code != 404 {
		t.Fatalf("missing route returned %d", response.Code)
	}
	for _, event := range store.events {
		if event.Status != 404 || event.Route != "unmatched" || event.Outcome != domainaudit.OutcomeFailed {
			t.Fatalf("%+v", event)
		}
	}
}
