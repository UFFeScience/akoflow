package eventloop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

const EventWorkflowExpansionRequested = "workflow.expansion.requested"

type WorkflowExpansionRunner interface {
	Apply(context.Context, domain.ExpansionRequest) (*domain.WorkflowExpansion, error)
}

type WorkflowExpansionHandler struct{ runner WorkflowExpansionRunner }

func NewWorkflowExpansionHandler(runner WorkflowExpansionRunner) *WorkflowExpansionHandler {
	return &WorkflowExpansionHandler{runner: runner}
}

func (h *WorkflowExpansionHandler) Handle(ctx context.Context, job domainqueue.Job) error {
	if h.runner == nil {
		return fmt.Errorf("workflow expansion service is not configured")
	}
	var request domain.ExpansionRequest
	if err := json.Unmarshal(job.Payload, &request); err != nil {
		return fmt.Errorf("decode workflow expansion request: %w", err)
	}
	_, err := h.runner.Apply(ctx, request)
	return err
}
