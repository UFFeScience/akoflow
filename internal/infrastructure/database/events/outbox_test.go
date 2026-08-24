package events

import (
	"context"
	"database/sql"
	"testing"

	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

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
