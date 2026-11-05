package queue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

func TestAnyNewCommandTypeHasAuditedRequestRetryAndCompletion(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	job, err := domainqueue.New("new-flow", "brandnew.flow.requested", []byte("secret-body"), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	job.IdempotencyKey = "new-flow-idempotency"
	stored, err := repo.Publish(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Publish(ctx, job); err != nil {
		t.Fatal(err)
	}
	leased, err := repo.Lease(ctx, "worker", []string{"new-flow"}, 1, time.Minute)
	if err != nil || len(leased) != 1 {
		t.Fatalf("%+v %v", leased, err)
	}
	if err := repo.Retry(ctx, stored.ID, "worker", errors.New("secret-error"), time.Now().UTC().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	leased, err = repo.Lease(ctx, "worker", []string{"new-flow"}, 1, time.Minute)
	if err != nil || len(leased) != 1 {
		t.Fatalf("%+v %v", leased, err)
	}
	if err := repo.Complete(ctx, stored.ID, "worker", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("SELECT event_type,metadata FROM audit_events WHERE source='event_queue'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var kind, metadata string
		if err := rows.Scan(&kind, &metadata); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(metadata, "secret") || !strings.Contains(metadata, "brandnew.flow.requested") {
			t.Fatal(metadata)
		}
		count++
	}
	if count != 5 {
		t.Fatalf("expected pending, lease1, retry1, lease2, complete2; got %d", count)
	}
}

func TestCommandAuditFailureRollsBackPublish(t *testing.T) {
	repo, db := setup(t)
	if _, err := db.Exec("DROP TABLE audit_events"); err != nil {
		t.Fatal(err)
	}
	job, err := domainqueue.New("flow", "flow.requested", nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Publish(context.Background(), job); err == nil {
		t.Fatal("missing audit error")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM queue_jobs WHERE id=?", job.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("%d %v", count, err)
	}
}
