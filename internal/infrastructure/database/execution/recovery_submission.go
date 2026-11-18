package execution

import (
	"context"
	"encoding/json"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	dbqueue "github.com/UFFeScience/akoflow/internal/infrastructure/database/queue"
)

func (r *Repository) SubmitRecovery(ctx context.Context, job domainqueue.Job, mode string) (domainqueue.Job, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domainqueue.Job{}, err
	}
	defer tx.Rollback()
	stored, inserted, err := dbqueue.PublishIn(ctx, tx, job)
	if err != nil {
		return domainqueue.Job{}, err
	}
	if !inserted {
		var request ports.ExecutionRequest
		if stored.AggregateID != job.AggregateID || json.Unmarshal(stored.Payload, &request) != nil ||
			request.Recovery == nil || request.Recovery.Mode != mode {
			return domainqueue.Job{}, ports.ErrExecutionSubmissionConflict
		}
		return stored, tx.Commit()
	}
	if err := resumeRunIn(ctx, tx, job.AggregateID); err != nil {
		return domainqueue.Job{}, err
	}
	return stored, tx.Commit()
}
