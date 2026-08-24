package eventloop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

const EventActivityExecutionRequested = "activity.execution.requested"

type ActivityStarter interface {
	Start(context.Context, domain.ActivityExecutionContext) (domain.ActivityHandle, error)
}

type ActivityExecutionPayload struct {
	Execution domain.ActivityExecutionContext `json:"execution"`
}

type ActivityExecutionHandler struct{ starter ActivityStarter }

func NewActivityExecutionHandler(starter ActivityStarter) *ActivityExecutionHandler {
	return &ActivityExecutionHandler{starter: starter}
}

func (h *ActivityExecutionHandler) Handle(ctx context.Context, job domainqueue.Job) error {
	if h.starter == nil {
		return fmt.Errorf("activity execution service is not configured")
	}
	var payload ActivityExecutionPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode activity execution: %w", err)
	}
	if payload.Execution.Activity.ID == "" || payload.Execution.Run.ID == "" {
		return fmt.Errorf("activity and execution run are required")
	}
	_, err := h.starter.Start(ctx, payload.Execution)
	return err
}
