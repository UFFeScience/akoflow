package queue

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func setup(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repository, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	return repository, db
}

func newJob(t *testing.T, key string) domainqueue.Job {
	t.Helper()
	job, err := domainqueue.New(domainqueue.CategoryExecution, "test.event", []byte(`{}`), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	job.IdempotencyKey = key
	job.MaxAttempts = 2
	return job
}

func TestPublishIsIdempotentAndLeaseCompletes(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	first, err := repository.Publish(ctx, newJob(t, "same"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.Publish(ctx, newJob(t, "same"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotency returned different jobs: %s %s", first.ID, second.ID)
	}

	jobs, err := repository.Lease(ctx, "worker", []string{domainqueue.CategoryExecution}, 2, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Attempts != 1 {
		t.Fatalf("unexpected leased jobs: %+v", jobs)
	}
	if err := repository.Complete(ctx, jobs[0].ID, "other", time.Now()); err == nil {
		t.Fatal("another owner must not complete the lease")
	}
	if err := repository.Complete(ctx, jobs[0].ID, "worker", time.Now()); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.FindByID(ctx, jobs[0].ID)
	if err != nil || stored.Status != domainqueue.StatusCompleted {
		t.Fatalf("unexpected stored job: %+v %v", stored, err)
	}
}

func TestRetryEventuallyFailsAndExpiredLeaseReturns(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	job, err := repository.Publish(ctx, newJob(t, "retry"))
	if err != nil {
		t.Fatal(err)
	}
	leased, err := repository.Lease(ctx, "worker", nil, 1, time.Millisecond)
	if err != nil || len(leased) != 1 {
		t.Fatalf("lease: %+v %v", leased, err)
	}
	if err := repository.Retry(ctx, job.ID, "worker", errors.New("boom"), time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	leased, err = repository.Lease(ctx, "worker", nil, 1, time.Minute)
	if err != nil || len(leased) != 1 {
		t.Fatalf("second lease: %+v %v", leased, err)
	}
	if err := repository.Retry(ctx, job.ID, "worker", errors.New("again"), time.Now()); err != nil {
		t.Fatal(err)
	}
	stored, _ := repository.FindByID(ctx, job.ID)
	if stored.Status != domainqueue.StatusFailed {
		t.Fatalf("expected failed, got %s", stored.Status)
	}

	expiring, _ := repository.Publish(ctx, newJob(t, "expired"))
	_, _ = repository.Lease(ctx, "dead-worker", nil, 1, time.Nanosecond)
	time.Sleep(time.Millisecond)
	count, err := repository.ReleaseExpired(ctx, time.Now().UTC())
	if err != nil || count != 1 {
		t.Fatalf("release expired: %d %v", count, err)
	}
	stored, _ = repository.FindByID(ctx, expiring.ID)
	if stored.Status != domainqueue.StatusPending {
		t.Fatalf("expected pending, got %s", stored.Status)
	}
}
