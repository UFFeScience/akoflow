package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func TestRepositoryRecordsAndFiltersAuditEvents(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	event := domainaudit.Event{ID: "audit-1", EventType: "resource.discovery.completed", EnvironmentID: "env",
		ConnectionID: "ssh", Outcome: domainaudit.OutcomeSucceeded, Summary: "Discovered 3 nodes",
		Metadata: map[string]any{"nodeCount": 3}, OccurredAt: time.Now().UTC()}
	if err := repository.RecordAuditEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListAuditEvents(context.Background(), domainaudit.Filter{EnvironmentID: "env", Limit: 10})
	if err != nil || len(items) != 1 || items[0].Metadata["nodeCount"] != float64(3) {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	items, err = repository.ListAuditEvents(context.Background(), domainaudit.Filter{ResourceID: "other"})
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}
