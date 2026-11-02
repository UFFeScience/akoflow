package queue

import (
	"context"
	"strings"
	"testing"
	"time"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

func TestExpiredLeaseCannotReplaySingleAttemptExecution(t *testing.T) {
	repository, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	job := executionJob(t, "real-no-replay", "real", "hpc")
	job.MaxAttempts = 1
	stored, err := repository.Publish(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	leased, err := repository.Lease(ctx, "worker", nil, 1, time.Millisecond)
	if err != nil || len(leased) != 1 {
		t.Fatalf("initial lease: %v %v", leased, err)
	}
	if _, err := repository.ReleaseExpired(ctx, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	terminal, err := repository.FindByID(ctx, stored.ID)
	if err != nil || terminal.Status != domainqueue.StatusFailed || terminal.CompletedAt == nil || !strings.Contains(terminal.LastError, "explicit recovery required") {
		t.Fatalf("expired execution not terminal: job=%+v err=%v", terminal, err)
	}
	replayed, err := repository.Lease(ctx, "another-worker", nil, 1, time.Minute)
	if err != nil || len(replayed) != 0 {
		t.Fatalf("execution replayed after attempt limit: %v %v", replayed, err)
	}
}
