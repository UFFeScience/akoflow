package queue

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	dbaudit "github.com/UFFeScience/akoflow/internal/infrastructure/database/audit"
)

// RecordLifecycle stores queue metadata only, never payload or last_error.
func RecordLifecycle(ctx context.Context, tx dbaudit.Execer, job domainqueue.Job) error {
	outcome := domainaudit.OutcomeStarted
	switch job.Status {
	case domainqueue.StatusCompleted:
		outcome = domainaudit.OutcomeSucceeded
	case domainqueue.StatusFailed, domainqueue.StatusCancelled:
		outcome = domainaudit.OutcomeFailed
	}
	kind, id := "queue_job", job.ID
	runID, operationID, sessionID, activityID := "", "", "", ""
	switch job.AggregateType {
	case "execution_run":
		kind, id, runID = "execution_run", job.AggregateID, job.AggregateID
	case "cloud_operation":
		kind, id, operationID = "cloud_operation", job.AggregateID, job.AggregateID
	case "activity_execution":
		if reader, ok := tx.(interface {
			QueryRowContext(context.Context, string, ...any) *sql.Row
		}); ok {
			err := reader.QueryRowContext(ctx, "SELECT execution_run_id,activity_id FROM task_executions WHERE id=?", job.AggregateID).Scan(&runID, &activityID)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if runID != "" {
				kind, id = "execution_run", runID
			}
		}
	case "planning_session":
		kind, id, sessionID = "planning_session", job.AggregateID, job.AggregateID
	}
	return dbaudit.AppendLifecycle(ctx, tx, domainaudit.Event{
		ID:        fmt.Sprintf("queue:%s:%d:%s", job.ID, job.Attempts, job.Status),
		EventType: "queue.job." + string(job.Status), Attempt: job.Attempts,
		ActorID: "daemon", ActorType: "service_client", AuthOutcome: "internal",
		Source: "event_queue", Transport: "internal", CorrelationID: "queue:" + job.ID,
		ExecutionID: runID, ActivityID: activityID, OperationID: operationID, SessionID: sessionID,
		Outcome: outcome, OccurredAt: time.Now().UTC(),
		Metadata: map[string]any{"entityType": job.AggregateType, "entityId": job.AggregateID, "commandType": job.Type, "commandId": job.ID, "category": job.Category},
	}, kind, id)
}

func (r *Repository) auditMutation(ctx context.Context, query string, args ...any) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, query+" RETURNING id,category,event_type,aggregate_type,aggregate_id,status,attempts", args...)
	if err != nil {
		return 0, err
	}
	jobs := []domainqueue.Job{}
	for rows.Next() {
		var job domainqueue.Job
		if err := rows.Scan(&job.ID, &job.Category, &job.Type, &job.AggregateType, &job.AggregateID, &job.Status, &job.Attempts); err != nil {
			rows.Close()
			return 0, err
		}
		jobs = append(jobs, job)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		if err := RecordLifecycle(ctx, tx, job); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(jobs)), nil
}
