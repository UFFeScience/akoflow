package execution

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"math"
	"runtime"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

var _ ports.LocalAdmissionStore = (*Repository)(nil)

func (r *Repository) ReserveLocalActivity(
	ctx context.Context,
	task *domain.TaskExecution,
	requested, limit ports.LocalActivityCapacity,
) (bool, error) {
	if requested.CPU <= 0 || requested.MemoryBytes <= 0 || math.IsNaN(requested.CPU) || math.IsInf(requested.CPU, 0) {
		return false, fmt.Errorf("invalid local activity capacity")
	}
	if requested.CPU > limit.CPU || requested.MemoryBytes > limit.MemoryBytes {
		return false, fmt.Errorf("activity requires more CPU or memory than the shared local budget")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	cancelled, err := runIsCancelled(ctx, tx, task.ExecutionRunID)
	if err != nil || cancelled {
		return false, err
	}
	var reconciling bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM task_executions
		WHERE execution_run_id=? AND activity_id=? AND attempt=?
		AND json_extract(metadata, '$.localNeedsReconciliation')=1)`,
		task.ExecutionRunID, task.ActivityID, task.Attempt).Scan(&reconciling); err != nil || reconciling {
		return false, err
	}
	used, err := localCapacityInUse(ctx, tx, *task)
	if err != nil {
		return false, err
	}
	if used.CPU > limit.CPU-requested.CPU || used.MemoryBytes > limit.MemoryBytes-requested.MemoryBytes {
		return false, nil
	}
	updated := *task
	updated.Status = domain.TaskPreparing
	updated.Metadata = maps.Clone(task.Metadata)
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]any)
	}
	updated.Metadata["localReservedCpu"] = requested.CPU
	updated.Metadata["localReservedMemoryBytes"] = requested.MemoryBytes
	if err := saveTask(ctx, tx, updated); err != nil {
		return false, err
	}
	if err := appendTaskEvent(ctx, tx, updated); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	*task = updated
	return true, nil
}

func localCapacityInUse(ctx context.Context, tx *sql.Tx, task domain.TaskExecution) (ports.LocalActivityCapacity, error) {
	var used ports.LocalActivityCapacity
	err := tx.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(json_extract(t.metadata, '$.localReservedCpu')), 0),
		COALESCE(SUM(json_extract(t.metadata, '$.localReservedMemoryBytes')), 0)
	FROM task_executions t
	LEFT JOIN activity_handles h ON h.id = t.execution_run_id || ':' || t.activity_id ||
		CASE WHEN t.attempt > 1 THEN ':attempt-' || t.attempt ELSE '' END
	WHERE NOT (t.execution_run_id = ? AND t.activity_id = ? AND t.attempt = ?)
	AND (h.status IN ('starting','running') OR json_extract(h.metadata, '$.runtimeReconciliationPending') = 1 OR
		(h.id IS NULL AND t.status IN ('preparing','running')))`,
		task.ExecutionRunID, task.ActivityID, task.Attempt).Scan(&used.CPU, &used.MemoryBytes)
	return used, err
}

func (r *Repository) MarkLocalActivityLaunching(ctx context.Context, runID, activityID string, attempt int, docker bool) error {
	result, err := r.db.ExecContext(ctx, `UPDATE task_executions
	SET metadata = json_set(json_remove(metadata, '$.localNeedsReconciliation'), '$.localLaunchMayExist', true, '$.localDockerLaunch', ?, '$.localNativeGated', ?)
	WHERE execution_run_id = ? AND activity_id = ? AND attempt = ? AND status = 'preparing'
	AND json_extract(metadata, '$.localReservedCpu') > 0`, docker, !docker && runtime.GOOS == "linux", runID, activityID, attempt)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return fmt.Errorf("local capacity reservation no longer permits launching")
	}
	return err
}

func (r *Repository) RecoverLocalPreparations(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE task_executions
	SET status='queued', metadata=json_set(
		json_remove(metadata, '$.localReservedCpu', '$.localReservedMemoryBytes'),
		'$.queueReason', 'waiting to resume after daemon restart')
	WHERE status='preparing' AND json_extract(metadata, '$.localReservedCpu') > 0
	AND COALESCE(json_extract(metadata, '$.localLaunchMayExist'), 0) = 0
	AND NOT EXISTS (
		SELECT 1 FROM activity_handles h WHERE h.id = task_executions.execution_run_id || ':' ||
		task_executions.activity_id || CASE WHEN task_executions.attempt > 1 THEN ':attempt-' || task_executions.attempt ELSE '' END
	)`)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE task_executions
		SET metadata=json_set(metadata, '$.localNeedsReconciliation', true)
		WHERE json_extract(metadata, '$.localReservedCpu') > 0 AND json_extract(metadata, '$.localLaunchMayExist') = 1`); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListLocalReconciliations(ctx context.Context) ([]ports.LocalReconciliation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id,t.execution_run_id,t.activity_id,t.attempt,
		COALESCE(json_extract(t.metadata, '$.localDockerLaunch'), 0),
		COALESCE(json_extract(t.metadata, '$.localNativeGated'), 0),
		COALESCE(json_extract(t.metadata, '$.localNativePid'), h.external_id, ''),
		COALESCE(json_extract(t.metadata, '$.localNativeIdentity'), json_extract(h.metadata, '$.nativeProcessIdentity'), '')
		FROM task_executions t
		LEFT JOIN activity_handles h ON h.id = t.execution_run_id || ':' || t.activity_id ||
			CASE WHEN t.attempt > 1 THEN ':attempt-' || t.attempt ELSE '' END
		WHERE json_extract(t.metadata, '$.localNeedsReconciliation') = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []ports.LocalReconciliation
	for rows.Next() {
		var task ports.LocalReconciliation
		if err := rows.Scan(&task.TaskID, &task.RunID, &task.ActivityID, &task.Attempt,
			&task.Docker, &task.NativeGated, &task.ProcessID, &task.ProcessIdentity); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *Repository) ReleaseReconciledLocalActivity(ctx context.Context, taskID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE task_executions SET metadata=json_remove(metadata,
		'$.localReservedCpu', '$.localReservedMemoryBytes', '$.localLaunchMayExist', '$.localDockerLaunch', '$.localNeedsReconciliation',
        '$.localNativeGated', '$.localNativePid', '$.localNativeIdentity')
		WHERE id = ? AND json_extract(metadata, '$.localNeedsReconciliation') = 1`, taskID)
	return err
}

func (r *Repository) RecordNativeLaunch(ctx context.Context, runID, activityID string, attempt int, pid, identity string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE task_executions
		SET metadata=json_set(metadata, '$.localNativePid', ?, '$.localNativeIdentity', ?)
		WHERE execution_run_id=? AND activity_id=? AND attempt=? AND status='preparing'
		AND json_extract(metadata, '$.localNativeGated')=1`, pid, identity, runID, activityID, attempt)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return fmt.Errorf("native process cannot start without a durable capacity reservation")
	}
	return err
}
