package eventloop

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"testing"
)

type expansionRunnerStub struct{ calls int }

func (s *expansionRunnerStub) Apply(context.Context, domain.ExpansionRequest) (*domain.WorkflowExpansion, error) {
	s.calls++
	return &domain.WorkflowExpansion{}, nil
}

func TestWorkflowExpansionHandlerDispatchesPersistentPayload(t *testing.T) {
	runner := &expansionRunnerStub{}
	handler := NewWorkflowExpansionHandler(runner)
	err := handler.Handle(context.Background(), domainqueue.Job{Payload: []byte(`{"id":"r","workflowVersionId":"v","sourceActivityId":"a","sourceEventId":"e","sequence":1,"activities":[{"key":"x","activity":{}}]}`)})
	if err != nil || runner.calls != 1 {
		t.Fatalf("unexpected dispatch: calls=%d err=%v", runner.calls, err)
	}
}
