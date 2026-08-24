package eventloop

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type starterFake struct {
	execution domain.ActivityExecutionContext
}

func (f *starterFake) Start(_ context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	f.execution = execution
	return domain.ActivityHandle{ID: "handle"}, nil
}

func TestActivityExecutionHandlerStartsUnifiedActivity(t *testing.T) {
	payload, err := json.Marshal(ActivityExecutionPayload{Execution: domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, Activity: domain.Activity{ID: "activity"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	starter := &starterFake{}
	handler := NewActivityExecutionHandler(starter)
	if err := handler.Handle(context.Background(), domainqueue.Job{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if starter.execution.Activity.ID != "activity" || starter.execution.Run.ID != "run" {
		t.Fatal("execution context was not forwarded")
	}
}

func TestActivityExecutionHandlerRejectsIncompletePayload(t *testing.T) {
	handler := NewActivityExecutionHandler(&starterFake{})
	if err := handler.Handle(context.Background(), domainqueue.Job{Payload: []byte(`{}`)}); err == nil {
		t.Fatal("incomplete execution must fail")
	}
	if err := NewActivityExecutionHandler(nil).Handle(context.Background(), domainqueue.Job{}); err == nil {
		t.Fatal("missing service must fail")
	}
}
