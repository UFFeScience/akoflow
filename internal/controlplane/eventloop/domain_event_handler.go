package eventloop

import (
	"context"
	"encoding/json"
	"fmt"

	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

// DomainEventHandler acknowledges durable outbox deliveries. The immutable
// event remains in domain_events; additional subscribers can project it later.
type DomainEventHandler struct{}

func (DomainEventHandler) Handle(_ context.Context, job domainqueue.Job) error {
	var event domainevents.Event
	if err := json.Unmarshal(job.Payload, &event); err != nil {
		return fmt.Errorf("decode domain event: %w", err)
	}
	if event.ID == "" || event.Type != job.Type || event.AggregateID == "" {
		return fmt.Errorf("invalid domain event envelope")
	}
	return nil
}
