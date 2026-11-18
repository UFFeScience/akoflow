package supportbundle

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type diagnosticAudit struct {
	events []domainaudit.Event
	err    error
}

func (a diagnosticAudit) RecordAuditEvent(context.Context, domainaudit.Event) error { return nil }

func (a diagnosticAudit) ListAuditEvents(_ context.Context, filter domainaudit.Filter) ([]domainaudit.Event, error) {
	if filter.Limit != 200 {
		return nil, errors.New("missing size limit")
	}
	return a.events, a.err
}

func TestSupportObservationsExcludeCredentialsIdentifiersAndUnboundedEvents(t *testing.T) {
	to := time.Now().UTC()
	from := to.Add(-time.Hour)
	collected := domainaudit.Event{
		OccurredAt: to.Add(-time.Minute), EventType: "execution.run.failed", Outcome: domainaudit.OutcomeFailed,
		ActorID: "sensitive-user", ExecutionID: "sensitive-run", Route: "https://sensitive-url",
		Summary: "sensitive command and access token", Metadata: map[string]any{"secret": "sensitive-value"},
	}
	events := []domainaudit.Event{{OccurredAt: to, EventType: "execution.run.sensitive", Outcome: domainaudit.OutcomeFailed}}
	for range 250 {
		events = append(events, collected)
	}
	report := (Collector{Version: "test", Audit: diagnosticAudit{events: events}}).
		Collect(context.Background(), map[string]bool{"system": true, "operations": true}, from, to)
	if report.System == nil || report.Operations == nil || len(*report.Operations) != 200 {
		t.Fatalf("unexpected support report: %+v", report)
	}
	bytes, err := json.Marshal(report)
	if err != nil || strings.Contains(string(bytes), "sensitive") {
		t.Fatalf("unsafe diagnostic report: %s %v", bytes, err)
	}
}

func TestSupportObservationsRetainPartialCollections(t *testing.T) {
	collector := Collector{Version: "test", Audit: diagnosticAudit{err: errors.New("sensitive failure")}}
	result := collector.Collect(context.Background(), map[string]bool{"system": true, "operations": true}, time.Now().Add(-time.Hour), time.Now())
	if result.System == nil || result.Operations != nil || len(result.Failures) != 1 || result.Failures[0].Code != "unavailable" {
		t.Fatalf("unexpected partial report: %+v", result)
	}
}
