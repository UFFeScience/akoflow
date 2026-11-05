package execution

import (
	"context"
	"database/sql"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func cancelTasksWithAudit(ctx context.Context, tx *sql.Tx, runID, reason string) error {
	rows, err := tx.QueryContext(ctx, `UPDATE task_executions SET status='cancelled',
 finished_at=CASE WHEN finished_at=0 THEN ? ELSE finished_at END,
 failure_reason=CASE WHEN failure_reason='' THEN ? ELSE failure_reason END
 WHERE execution_run_id=? AND status IN ('blocked','ready','queued','preparing','running')
 RETURNING id,execution_run_id,activity_id,attempt`,
		float64(time.Now().UnixNano())/float64(time.Second), reason, runID)
	if err != nil {
		return err
	}
	tasks := []domain.TaskExecution{}
	for rows.Next() {
		task := domain.TaskExecution{Status: domain.TaskCancelled}
		if err := rows.Scan(&task.ID, &task.ExecutionRunID, &task.ActivityID, &task.Attempt); err != nil {
			rows.Close()
			return err
		}
		tasks = append(tasks, task)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := auditTaskState(ctx, tx, task); err != nil {
			return err
		}
	}
	return nil
}
