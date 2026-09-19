package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

func executionJob(t *testing.T, key, mode string, environments ...string) domainqueue.Job {
	t.Helper()
	resources := make([]map[string]string, 0, len(environments))
	assignments := make([]map[string]string, 0, len(environments))
	for index, environmentID := range environments {
		resourceID := fmt.Sprintf("resource-%s-%d", key, index)
		resources = append(resources, map[string]string{
			"id": resourceID, "environmentVersionId": environmentID,
		})
		assignments = append(assignments, map[string]string{"resourceId": resourceID})
	}
	payload, err := json.Marshal(map[string]any{
		"run":       map[string]string{"id": key, "mode": mode},
		"plan":      map[string]any{"assignments": assignments},
		"resources": resources,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, err := domainqueue.New(domainqueue.CategoryExecution, executionRunRequested, payload, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	job.IdempotencyKey = key
	return job
}

func TestRepositoryConstructionAndClose(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("nil database must be rejected")
	}
	repository, db := setup(t)
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("non-owned database was closed: %v", err)
	}
	repository.owned = true
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
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

func TestPublishWithoutIdempotencyAndValidateLeaseArguments(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	invalid := newJob(t, "")
	invalid.ID = ""
	if _, err := repository.Publish(ctx, invalid); err == nil {
		t.Fatal("invalid job must fail")
	}
	job := newJob(t, "")
	job.AvailableAt = time.Time{}
	job.CreatedAt = time.Time{}
	published, err := repository.Publish(ctx, job)
	if err != nil || published.ID != job.ID || published.AvailableAt.IsZero() || published.CreatedAt.IsZero() {
		t.Fatalf("published=%+v err=%v", published, err)
	}
	for _, args := range []struct {
		owner    string
		limit    int
		duration time.Duration
	}{{"", 1, time.Second}, {"worker", 0, time.Second}, {"worker", 1, 0}} {
		if _, err := repository.Lease(ctx, args.owner, nil, args.limit, args.duration); err == nil {
			t.Fatalf("invalid lease accepted: %+v", args)
		}
	}
	missing, err := repository.FindByID(ctx, "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
}

func TestRenewCancelAndRetryWithoutCause(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	job, err := repository.Publish(ctx, newJob(t, "lifecycle"))
	if err != nil {
		t.Fatal(err)
	}
	leased, err := repository.Lease(ctx, "worker", nil, 1, time.Minute)
	if err != nil || len(leased) != 1 {
		t.Fatalf("leased=%+v err=%v", leased, err)
	}
	expires := time.Now().UTC().Add(2 * time.Minute)
	if err := repository.RenewLease(ctx, job.ID, "other", expires); err == nil {
		t.Fatal("another owner must not renew")
	}
	if err := repository.RenewLease(ctx, job.ID, "worker", expires); err != nil {
		t.Fatal(err)
	}
	if err := repository.Retry(ctx, job.ID, "worker", nil, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := repository.Cancel(ctx, job.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.FindByID(ctx, job.ID)
	if err != nil || stored == nil || stored.Status != domainqueue.StatusCancelled || stored.CompletedAt == nil {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	if err := repository.Cancel(ctx, "missing", time.Now()); err == nil {
		t.Fatal("missing job cancel must fail")
	}
}

func TestLeaseSerializesRealExecutionsByEnvironment(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	for _, job := range []domainqueue.Job{
		executionJob(t, "first-a", "real", "environment-a"),
		executionJob(t, "second-a", "real", "environment-a"),
		executionJob(t, "first-b", "real", "environment-b"),
	} {
		if _, err := repository.Publish(ctx, job); err != nil {
			t.Fatal(err)
		}
	}

	leased, err := repository.Lease(ctx, "worker", []string{domainqueue.CategoryExecution}, 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(leased) != 2 || leased[0].IdempotencyKey != "first-a" || leased[1].IdempotencyKey != "first-b" {
		t.Fatalf("leased=%+v", leased)
	}
	if err := repository.Complete(ctx, leased[0].ID, "worker", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	next, err := repository.Lease(ctx, "worker-2", []string{domainqueue.CategoryExecution}, 3, time.Minute)
	if err != nil || len(next) != 1 || next[0].IdempotencyKey != "second-a" {
		t.Fatalf("next=%+v err=%v", next, err)
	}
}

func TestLeaseAcquiresAllExecutionEnvironmentsAtomically(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	jobs := []domainqueue.Job{
		executionJob(t, "multi", "real", "environment-a", "environment-b"),
		executionJob(t, "only-a", "real", "environment-a"),
		executionJob(t, "only-c", "real", "environment-c"),
	}
	for _, job := range jobs {
		if _, err := repository.Publish(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	leased, err := repository.Lease(ctx, "worker", []string{domainqueue.CategoryExecution}, 3, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(leased) != 2 || leased[0].IdempotencyKey != "multi" || leased[1].IdempotencyKey != "only-c" {
		t.Fatalf("leased=%+v", leased)
	}
}

func TestLeaseDoesNotSerializeSimulationExecutions(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	for _, key := range []string{"simulation-a", "simulation-b"} {
		if _, err := repository.Publish(ctx, executionJob(t, key, "simulation", "environment-a")); err != nil {
			t.Fatal(err)
		}
	}
	leased, err := repository.Lease(ctx, "worker", []string{domainqueue.CategoryExecution}, 2, time.Minute)
	if err != nil || len(leased) != 2 {
		t.Fatalf("leased=%+v err=%v", leased, err)
	}
}

func TestLeaseSerializesDifferentVersionsOfSameEnvironment(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO environments(id,name) VALUES('environment','Environment')`); err != nil {
		t.Fatal(err)
	}
	for version, id := range []string{"environment-v1", "environment-v2"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO environment_versions(
			id,environment_id,version,status,network_model,interference_model,cost_model,configuration_hash
		) VALUES(?,'environment',?,'published','measured','none','none',?)`, id, version+1, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, job := range []domainqueue.Job{
		executionJob(t, "version-1", "real", "environment-v1"),
		executionJob(t, "version-2", "real", "environment-v2"),
	} {
		if _, err := repository.Publish(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	leased, err := repository.Lease(ctx, "worker", []string{domainqueue.CategoryExecution}, 2, time.Minute)
	if err != nil || len(leased) != 1 || leased[0].IdempotencyKey != "version-1" {
		t.Fatalf("leased=%+v err=%v", leased, err)
	}
}
