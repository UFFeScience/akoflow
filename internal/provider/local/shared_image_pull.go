package local

import (
	"context"
	"errors"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

type imagePullKey struct {
	runID, image, platform string
}

type imagePull struct {
	operationID string
	done        chan struct{}
	output      []byte
	err         error
	finishedAt  time.Time
	status      string
}

func (a *Adapter) waitForPull(ctx context.Context, execution domain.ActivityExecutionContext, pull *imagePull) ([]byte, error) {
	observation := domainexecution.ImagePreparationObservation{
		OperationID: pull.operationID, ActivityID: execution.Activity.ID, Attempt: max(1, execution.Attempt),
		Kind: domainexecution.ImagePreparationPull, StartedAt: time.Now().UTC(), Status: "running", Shared: true,
	}
	if err := a.savePreparation(ctx, execution.Run.ID, observation); err != nil {
		return nil, err
	}
	var output []byte
	var err error
	select {
	case <-ctx.Done():
		end := time.Now().UTC()
		observation.FinishedAt, observation.Status = &end, "cancelled"
		err = ctx.Err()
	case <-pull.done:
		finish := maxTime(observation.StartedAt, pull.finishedAt)
		observation.FinishedAt, observation.Status = &finish, pull.status
		output, err = pull.output, pull.err
	}
	return output, errors.Join(err, a.savePreparation(ctx, execution.Run.ID, observation))
}

func maxTime(first, second time.Time) time.Time {
	if second.After(first) {
		return second
	}
	return first
}
