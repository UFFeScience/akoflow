package eventloop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type failedLeaseQueue struct{ ports.QueueStore }

func (failedLeaseQueue) RenewLease(context.Context, string, string, time.Time) error {
	return errors.New("lease ownership lost")
}

func TestLeaseRenewalCancellationPreservesCause(t *testing.T) {
	loop := &Loop{queue: failedLeaseQueue{}, config: Config{Owner: "worker", LeaseDuration: 30 * time.Millisecond}}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	done := make(chan struct{})
	go loop.keepLeaseAlive(ctx, domainqueue.Job{ID: "execution-command"}, cancel, done)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lease renewal did not cancel handler")
	}
	if cause := context.Cause(ctx); cause == nil || !strings.Contains(cause.Error(), "execution-command") || !strings.Contains(cause.Error(), "lease ownership lost") {
		t.Fatalf("cancellation cause lost: %v", cause)
	}
}
