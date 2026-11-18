package execution

import (
	"bytes"
	"context"
	"database/sql"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	dbqueue "github.com/UFFeScience/akoflow/internal/infrastructure/database/queue"
)

func (r *Repository) SubmitRun(ctx context.Context, run domain.ExecutionRun, job domainqueue.Job) (domainqueue.Job, error) {
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
		if stored.AggregateID != run.ID || !bytes.Equal(stored.Payload, job.Payload) {
			return domainqueue.Job{}, ports.ErrExecutionSubmissionConflict
		}
		return stored, tx.Commit()
	}
	var id string
	err = tx.QueryRowContext(ctx, "SELECT id FROM execution_runs WHERE id=?", run.ID).Scan(&id)
	if err != sql.ErrNoRows {
		if err == nil {
			err = ports.ErrExecutionSubmissionConflict
		}
		return domainqueue.Job{}, err
	}
	if err := createRunIn(ctx, tx, run); err != nil {
		return domainqueue.Job{}, err
	}
	return stored, tx.Commit()
}
