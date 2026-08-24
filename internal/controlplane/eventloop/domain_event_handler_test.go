package eventloop

import (
	"context"
	"encoding/json"
	"testing"

	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

func TestDomainEventHandlerValidatesEnvelope(t *testing.T) {
	event := domainevents.Event{ID: "event", Type: domainevents.ExecutionStarted,
		AggregateType: "execution_run", AggregateID: "run"}
	payload, _ := json.Marshal(event)
	job := domainqueue.Job{Type: event.Type, Payload: payload}
	if err := (DomainEventHandler{}).Handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	job.Type = "different"
	if err := (DomainEventHandler{}).Handle(context.Background(), job); err == nil {
		t.Fatal("mismatched event type must fail")
	}
}
