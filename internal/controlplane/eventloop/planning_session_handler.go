package eventloop

import (
	"context"
	"encoding/json"
	"fmt"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

const EventPlanningSessionRequested = "planning.session.requested"

type PlanningRunner interface {
	Execute(context.Context, string) error
}

type PlanningSessionHandler struct{ runner PlanningRunner }

func NewPlanningSessionHandler(runner PlanningRunner) *PlanningSessionHandler {
	return &PlanningSessionHandler{runner: runner}
}

func (h *PlanningSessionHandler) Handle(ctx context.Context, job domainqueue.Job) error {
	if h.runner == nil {
		return fmt.Errorf("planning service is not configured")
	}
	var payload struct {
		PlanningSessionID string `json:"planningSessionId"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode planning request: %w", err)
	}
	if payload.PlanningSessionID == "" {
		return fmt.Errorf("planning session id is required")
	}
	return h.runner.Execute(ctx, payload.PlanningSessionID)
}
