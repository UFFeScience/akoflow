package imagepreparation

import (
	"context"
	"errors"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/google/uuid"
)

type targetKey struct{}

type target struct {
	recorder   ports.ImagePreparationRecorder
	runID      string
	activityID string
	attempt    int
}

func WithTarget(ctx context.Context, recorder ports.ImagePreparationRecorder, runID, activityID string, attempt int) context.Context {
	return context.WithValue(ctx, targetKey{}, target{recorder, runID, activityID, max(1, attempt)})
}

func Observe(ctx context.Context, kind domainexecution.ImagePreparationKind, action func() error) error {
	current, ok := ctx.Value(targetKey{}).(target)
	if !ok {
		return action()
	}
	observation := domainexecution.ImagePreparationObservation{
		OperationID: uuid.NewString(), ActivityID: current.activityID, Attempt: current.attempt,
		Kind: kind, StartedAt: time.Now().UTC(), Status: "running",
	}
	if kind != domainexecution.ImagePreparationCacheHit {
		if err := current.recorder.RecordImagePreparation(ctx, current.runID, observation); err != nil {
			return err
		}
	}
	err := action()
	end := time.Now().UTC()
	observation.FinishedAt, observation.Status = &end, "completed"
	if ctx.Err() != nil {
		observation.Status = "cancelled"
	} else if err != nil {
		observation.Status = "failed"
	}
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	return errors.Join(err, current.recorder.RecordImagePreparation(persist, current.runID, observation))
}
