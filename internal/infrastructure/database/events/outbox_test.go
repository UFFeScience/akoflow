package events

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

type failingExecer struct {
	calls  int
	failAt int
}

func (f *failingExecer) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	f.calls++
	if f.calls == f.failAt {
		return nil, errors.New("database unavailable")
	}
	return nil, nil
}

func TestAppendPersistsEventAndQueueDeliveryAtomically(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Append(context.Background(), tx, domainevents.Event{
		Type: domainevents.ExecutionStarted, AggregateType: "execution_run", AggregateID: "run",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var events, deliveries int
	_ = db.QueryRow(`SELECT COUNT(*) FROM domain_events`).Scan(&events)
	_ = db.QueryRow(`SELECT COUNT(*) FROM queue_jobs WHERE event_type='execution.run.started'`).Scan(&deliveries)
	if events != 1 || deliveries != 1 {
		t.Fatalf("events=%d deliveries=%d", events, deliveries)
	}
}

func TestAppendValidatesAndSerializesEvents(t *testing.T) {
	valid := domainevents.Event{ID: "event", Type: domainevents.ExecutionStarted, AggregateType: "execution_run", AggregateID: "run", OccurredAt: time.Unix(10, 0).UTC()}
	for _, event := range []domainevents.Event{
		{},
		{Type: valid.Type, AggregateType: valid.AggregateType},
		{Type: valid.Type, AggregateID: valid.AggregateID},
	} {
		if err := Append(context.Background(), &failingExecer{}, event); err == nil {
			t.Fatalf("invalid event accepted: %+v", event)
		}
	}
	invalidPayload := valid
	invalidPayload.Payload = map[string]any{"channel": make(chan int)}
	if err := Append(context.Background(), &failingExecer{}, invalidPayload); err == nil || !strings.Contains(err.Error(), "marshal domain event") {
		t.Fatalf("unexpected marshal error: %v", err)
	}
}

func TestAppendReportsEventAndQueueWriteFailures(t *testing.T) {
	event := domainevents.Event{Type: domainevents.ExecutionStarted, AggregateType: "execution_run", AggregateID: "run"}
	first := &failingExecer{failAt: 1}
	if err := Append(context.Background(), first, event); err == nil || !strings.Contains(err.Error(), "append domain event") {
		t.Fatalf("unexpected first write error: %v", err)
	}
	second := &failingExecer{failAt: 2}
	if err := Append(context.Background(), second, event); err == nil || !strings.Contains(err.Error(), "enqueue domain event") {
		t.Fatalf("unexpected second write error: %v", err)
	}
	if second.calls != 2 {
		t.Fatalf("calls=%d", second.calls)
	}
}
