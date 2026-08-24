package eventloop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

const EventExecutionRunRequested = "execution.run.requested"

type ExecutionRunner interface {
	Execute(context.Context, ports.ExecutionRequest) (domain.ExecutionTrace, error)
}

type ExecutionRunHandler struct{ runner ExecutionRunner }

func NewExecutionRunHandler(runner ExecutionRunner) *ExecutionRunHandler {
	return &ExecutionRunHandler{runner: runner}
}

func (h *ExecutionRunHandler) Handle(ctx context.Context, job domainqueue.Job) error {
	if h.runner == nil {
		return fmt.Errorf("execution supervisor is not configured")
	}
	var request ports.ExecutionRequest
	if err := json.Unmarshal(job.Payload, &request); err != nil {
		return fmt.Errorf("decode execution request: %w", err)
	}
	if request.Run.ID == "" {
		return fmt.Errorf("execution run id is required")
	}
	_, err := h.runner.Execute(ctx, request)
	return err
}
