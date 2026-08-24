package ports

import (
	"context"
	"time"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

// QueueStore is the durable boundary used by the event loop. Implementations
// own persistence and leasing; application services only express queue intent.
type QueueStore interface {
	Publish(context.Context, domainqueue.Job) (domainqueue.Job, error)
	Lease(context.Context, string, []string, int, time.Duration) ([]domainqueue.Job, error)
	RenewLease(context.Context, string, string, time.Time) error
	Complete(context.Context, string, string, time.Time) error
	Retry(context.Context, string, string, error, time.Time) error
	Cancel(context.Context, string, time.Time) error
	ReleaseExpired(context.Context, time.Time) (int64, error)
	FindByID(context.Context, string) (*domainqueue.Job, error)
}

type EventPublisher interface {
	Publish(context.Context, domainqueue.Job) (domainqueue.Job, error)
}
