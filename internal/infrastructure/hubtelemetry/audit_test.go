package hubtelemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain/audit"
)

func TestRecordAccessKeepsOnlyPublicRouteAndCorrelation(t *testing.T) {
	sender := New("https://example.test", "test")
	sender.SetConsent(APIAccess, true)
	sender.SetConsent(Diagnostics, true)
	correlation := uuid.New()
	entry := audit.Event{
		ID: uuid.NewString(), CorrelationID: correlation.String(), EventType: "api.access",
		Outcome: audit.OutcomeFailed, OccurredAt: time.Now(), Method: "GET",
		Route: "GET /akoflow-api/connections/{connectionId}", Status: http.StatusInternalServerError,
		ActorID: "private-actor", Summary: "private error text", ConnectionID: "private-id",
	}
	sender.RecordAccess(entry, "dev-env")
	for _, stream := range []Stream{APIAccess, Diagnostics} {
		pending := sender.streams[stream].pending
		if len(pending) != 1 {
			t.Fatalf("stream %s has %d events, want 1", stream, len(pending))
		}
		var event Event
		if err := json.Unmarshal(pending[0], &event); err != nil {
			t.Fatal(err)
		}
		if event.CorrelationID == nil || *event.CorrelationID != correlation {
			t.Fatal("missing correlation")
		}
		if stream == APIAccess && (len(event.Fields) != 4 || event.Fields["route"] != "/akoflow-api/connections/{connectionId}") {
			t.Fatalf("unexpected access fields: %+v", event.Fields)
		}
		if stream == Diagnostics && (len(event.Fields) != 0 || len(event.Fingerprint) != 64) {
			t.Fatalf("unexpected diagnostic: %+v", event)
		}
	}
}

type captureAudit struct {
	ports.AuditStore
}

func (captureAudit) RecordAuditEvent(context.Context, audit.Event) error { return nil }

func TestMCPDiagnosticsRequireAnInternalFailure(t *testing.T) {
	sender := New("https://example.test", "fixture")
	sender.SetConsent(Diagnostics, true)
	store := AuditStore{AuditStore: captureAudit{}, Sender: sender, Release: "dev-env"}
	entry := audit.Event{
		ID: uuid.NewString(), CorrelationID: uuid.NewString(), EventType: "mcp.request",
		Method: "tools/call", Outcome: audit.OutcomeFailed, OccurredAt: time.Now(),
		Summary: "private tool output",
	}
	if err := store.RecordAuditEvent(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	if len(sender.streams[Diagnostics].pending) != 0 {
		t.Fatal("expected MCP tool failure was collected")
	}
	entry.InternalFailure = true
	if err := store.RecordAuditEvent(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	pending := sender.streams[Diagnostics].pending
	if len(pending) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(pending))
	}
	var event Event
	if err := json.Unmarshal(pending[0], &event); err != nil {
		t.Fatal(err)
	}
	if event.Code != "mcp.internal_error" || event.CorrelationID == nil ||
		event.CorrelationID.String() != entry.CorrelationID || len(event.Fields) != 0 {
		t.Fatalf("unexpected MCP diagnostic: %+v", event)
	}
}
