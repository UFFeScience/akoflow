package audit

import (
	"context"
	"database/sql"
	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
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

func TestUnifiedAuditPaginationFiltersAndRedaction(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	ctx := context.Background()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	now := time.Now().UTC()
	for _, id := range []string{"a", "b", "c"} {
		event := domainaudit.Event{ID: id, EventType: "api.access", RequestID: "request", CorrelationID: "correlation",
			ActorID: "service", ActorType: "service_client", Method: "GET", Route: "GET /resource/{id}", Status: 200,
			Transport: "http", ExecutionID: "run", ActivityID: "activity", RuntimeID: "runtime", OperationID: "operation",
			Outcome: domainaudit.OutcomeSucceeded, Summary: "password=secret\nforged-line", OccurredAt: now,
			Metadata: map[string]any{"Authorization": "Bearer secret", "stdout": "secret", "body": "secret", "nodeCount": 3}}
		if err := repository.RecordAuditEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	filters := []domainaudit.Filter{
		{RequestID: "request"}, {CorrelationID: "correlation"}, {ActorID: "service"}, {Method: "GET"},
		{Route: "GET /resource/{id}"}, {Status: 200}, {Transport: "http"}, {ExecutionID: "run"},
		{ActivityID: "activity"}, {RuntimeID: "runtime"}, {OperationID: "operation"},
		{From: now.Add(-time.Second).Format(time.RFC3339Nano), To: now.Add(time.Second).Format(time.RFC3339Nano)},
	}
	for _, filter := range filters {
		items, err := repository.ListAuditEvents(ctx, filter)
		if err != nil || len(items) != 3 {
			t.Fatalf("filter=%+v items=%d err=%v", filter, len(items), err)
		}
		for _, e := range items {
			if e.Summary != "api.access" || len(e.Metadata) != 1 {
				t.Fatalf("sensitive metadata: %+v", e)
			}
		}
	}
	first, err := repository.ListAuditEvents(ctx, domainaudit.Filter{Limit: 2})
	if err != nil || len(first) != 2 || first[0].ID != "c" || first[1].ID != "b" {
		t.Fatalf("%+v %v", first, err)
	}
	next, err := repository.ListAuditEvents(ctx, domainaudit.Filter{Limit: 2, Before: first[1].ID})
	if err != nil || len(next) != 1 || next[0].ID != "a" {
		t.Fatalf("%+v %v", next, err)
	}
	missing, err := repository.ListAuditEvents(ctx, domainaudit.Filter{Status: 401})
	if err != nil || len(missing) != 0 {
		t.Fatalf("%+v %v", missing, err)
	}
	updated := first[0]
	updated.Outcome = domainaudit.OutcomeFailed
	updated.ActorID = "resolved"
	updated.ResourceID = "target"
	updated.Status = 500
	if err := repository.RecordAuditEvent(ctx, updated); err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListAuditEvents(ctx, domainaudit.Filter{ActorID: "resolved"})
	if err != nil || len(items) != 1 || items[0].ResourceID != "target" || items[0].Status != 500 {
		t.Fatalf("%+v %v", items, err)
	}
}

func TestAuditSanitizesControlCharactersAndBoundsFields(t *testing.T) {
	event := sanitize(domainaudit.Event{ID: "id\nforged", ActorID: "actor\r\nforged", EventType: "event", Summary: "secret", Metadata: map[string]any{"logReference": "line\x00\nforged", "nested": map[string]any{"secret": "value"}}})
	if event.ID != "idforged" || event.ActorID != "actorforged" || event.Metadata["logReference"] != "lineforged" || len(event.Metadata) != 1 {
		t.Fatalf("%+v", event)
	}
}

func TestAuditStoreFailureIsReturned(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := New(db).RecordAuditEvent(context.Background(), domainaudit.Event{ID: "id"}); err == nil {
		t.Fatal("storage error dropped")
	}
}

func TestAsyncLifecyclePreservesBoundRequestProvenance(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	ctx := context.Background()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	requestCtx := auditcontext.With(ctx, &auditcontext.Trace{RequestID: "request", CorrelationID: "correlation", ActorID: "real-subject", ActorType: "principal", AuthOutcome: "authenticated"})
	if err := Bind(requestCtx, tx, "execution_run", "run"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendLifecycle(ctx, tx, domainaudit.Event{ID: "event", EventType: "activity.started", ExecutionID: "run", Outcome: domainaudit.OutcomeStarted, OccurredAt: time.Now().UTC()}, "execution_run", "run"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	items, err := New(db).ListAuditEvents(ctx, domainaudit.Filter{RequestID: "request"})
	if err != nil || len(items) != 1 || items[0].ActorID != "real-subject" || items[0].CorrelationID != "correlation" {
		t.Fatalf("%+v %v", items, err)
	}
}

func TestEventFamilyFiltersKeepLegacyAndFutureEvents(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	ctx := context.Background()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	families := []string{"activity", "workflow", "execution", "connection", "console", "credential", "resource", "api", "queue", "planning", "storage", "future"}
	for _, family := range families {
		if err := repo.RecordAuditEvent(ctx, domainaudit.Event{ID: family, EventType: family + ".event", Outcome: domainaudit.OutcomeSucceeded, OccurredAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := repo.ListAuditEvents(ctx, domainaudit.Filter{})
	if err != nil || len(all) != len(families) {
		t.Fatalf("%d %v", len(all), err)
	}
	for _, family := range families {
		events, err := repo.ListAuditEvents(ctx, domainaudit.Filter{Category: family})
		if err != nil || len(events) != 1 || events[0].ID != family {
			t.Fatalf("%s: %+v %v", family, events, err)
		}
	}
	if _, err := repo.ListAuditEvents(ctx, domainaudit.Filter{Category: "api%' OR 1=1"}); err == nil {
		t.Fatal("invalid family accepted")
	}
}
