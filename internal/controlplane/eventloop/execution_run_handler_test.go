package eventloop

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type executionRunnerFake struct {
	request ports.ExecutionRequest
	err     error
}

func (f *executionRunnerFake) Execute(_ context.Context, request ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	f.request = request
	return domain.ExecutionTrace{}, f.err
}

func TestExecutionRunHandlerDelegatesDurableRequest(t *testing.T) {
	payload, _ := json.Marshal(ports.ExecutionRequest{Run: domain.ExecutionRun{ID: "run"}})
	runner := &executionRunnerFake{}
	if err := NewExecutionRunHandler(runner).Handle(context.Background(), domainqueue.Job{Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if runner.request.Run.ID != "run" {
		t.Fatal("request was not delegated")
	}
}

func TestExecutionRunHandlerValidatesConfigurationAndPayload(t *testing.T) {
	if err := NewExecutionRunHandler(nil).Handle(context.Background(), domainqueue.Job{}); err == nil {
		t.Fatal("missing supervisor must fail")
	}
	if err := NewExecutionRunHandler(&executionRunnerFake{}).Handle(context.Background(), domainqueue.Job{Payload: []byte(`{}`)}); err == nil {
		t.Fatal("missing run id must fail")
	}
}
