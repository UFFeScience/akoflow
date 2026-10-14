package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (s *Controller) persistUncertainStart(ctx context.Context, execution domain.ActivityExecutionContext, cause error) (domain.ActivityHandle, error) {
	var uncertain ports.RuntimeReconciliationFailure
	if !errors.As(cause, &uncertain) {
		return domain.ActivityHandle{}, cause
	}
	handle := domain.ActivityHandle{
		ID: execution.HandleID(), RunID: execution.Run.ID, ActivityID: execution.Activity.ID,
		ResourceID: execution.Resource.ID, RuntimeID: execution.RuntimeID,
		Status: domain.HandleStarting, StartedAt: float64(time.Now().UnixNano()) / float64(time.Second),
		Failure: cause.Error(), Metadata: uncertain.RuntimeReconciliationMetadata(),
	}
	handle.Metadata[ports.RuntimeReconciliationPending] = true
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := s.handles.Save(persist, handle); err != nil {
		return handle, fmt.Errorf("persist uncertain runtime start: %v: %w", err, cause)
	}
	return handle, cause
}
