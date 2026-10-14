package queue

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRenewLeaseReportsExpiredTerminalState(t *testing.T) {
	repo, db := setup(t)
	defer db.Close()
	ctx := context.Background()
	job := executionJob(t, "lease-diagnostic", "simulation")
	job.MaxAttempts = 1
	if _, err := repo.Publish(ctx, job); err != nil {
		t.Fatal(err)
	}
	leased, err := repo.Lease(ctx, "worker", nil, 1, time.Second)
	if err != nil || len(leased) != 1 {
		t.Fatalf("lease: %v %v", leased, err)
	}
	if _, err := repo.ReleaseExpired(ctx, time.Now().UTC().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	err = repo.RenewLease(ctx, job.ID, "worker", time.Now().UTC().Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "status=failed") || !strings.Contains(err.Error(), "ownerMatches=false") || !strings.Contains(err.Error(), "explicit recovery required") {
		t.Fatalf("missing diagnostic: %v", err)
	}
}
