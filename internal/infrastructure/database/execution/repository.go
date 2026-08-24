package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	dbevents "github.com/UFFeScience/akoflow/internal/infrastructure/database/events"
)

type Repository struct{ db *sql.DB }

var _ ports.ExecutionStore = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateRun(ctx context.Context, run domain.ExecutionRun) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO execution_runs (
		id, schedule_plan_id, mode, seed, status, environment_snapshot_id, started_at
	) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO NOTHING`, run.ID, run.SchedulePlanID, run.Mode, run.Seed,
		run.Status, run.EnvironmentSnapshotID)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed == 1 {
		if err := dbevents.Append(ctx, tx, domainevents.Event{
			Type: domainevents.ExecutionStarted, AggregateType: "execution_run", AggregateID: run.ID,
			Payload: map[string]any{"mode": run.Mode, "schedulePlanId": run.SchedulePlanID},
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) FindRun(ctx context.Context, id string) (*domain.ExecutionRun, error) {
	run, err := scanRunFeed(r.db.QueryRowContext(ctx, `SELECT * FROM (`+runFeedSelect+`) WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return run, err
}

func (r *Repository) ListRuns(ctx context.Context) ([]domain.ExecutionRun, error) {
	page, err := r.ListRunsPage(ctx, 1, 500, "", "", "")
	return page.Items, err
}

func (r *Repository) ListRunsPage(ctx context.Context, page, pageSize int, kind, mode, status string) (domain.ExecutionRunPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	conditions, arguments := runFeedFilters(kind, mode, status)
	filteredFeed := `SELECT * FROM (` + runFeedSelect + `)` + conditions
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+filteredFeed+`)`, arguments...).Scan(&total); err != nil {
		return domain.ExecutionRunPage{}, err
	}
	pageArguments := append(append([]any{}, arguments...), pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, filteredFeed+` ORDER BY started_at DESC, id DESC LIMIT ? OFFSET ?`, pageArguments...)
	if err != nil {
		return domain.ExecutionRunPage{}, err
	}
	defer rows.Close()
	runs := make([]domain.ExecutionRun, 0)
	for rows.Next() {
		run, err := scanRunFeed(rows)
		if err != nil {
			return domain.ExecutionRunPage{}, err
		}
		runs = append(runs, *run)
	}
	if err := rows.Err(); err != nil {
		return domain.ExecutionRunPage{}, err
	}
	return domain.ExecutionRunPage{Items: runs, Page: page, PageSize: pageSize, Total: total, HasNext: page*pageSize < total}, nil
}

func runFeedFilters(kind, mode, status string) (string, []any) {
	conditions := []string{}
	arguments := []any{}
	for column, value := range map[string]string{"kind": kind, "mode": mode, "status": status} {
		if value != "" && value != "all" {
			conditions = append(conditions, column+"=?")
			arguments = append(arguments, value)
		}
	}
	if len(conditions) == 0 {
		return "", arguments
	}
	return " WHERE " + strings.Join(conditions, " AND "), arguments
}

const runFeedSelect = `
	SELECT e.id, e.schedule_plan_id, e.mode, e.seed, e.status, e.environment_snapshot_id,
		e.makespan_seconds, e.cost, e.failure_reason,
		COALESCE(t.activity_count, 0), COALESCE(t.completed_count, 0),
		COALESCE(t.compute_seconds, 0), COALESCE(d.transfer_seconds, 0),
		COALESCE(t.queue_seconds, 0), COALESCE(t.interference_seconds, 0),
		COALESCE(t.overhead_seconds, 0), COALESCE(d.transferred_bytes, 0),
		'workflow' AS kind, e.id AS title, '' AS resource_id, '' AS runtime_id,
		'' AS connection_id, '' AS actor_id, '' AS command, e.started_at, e.finished_at
	FROM execution_runs e
	LEFT JOIN (
		SELECT execution_run_id, COUNT(*) AS activity_count,
			SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END) AS completed_count,
			SUM(runtime_seconds) AS compute_seconds, SUM(queue_seconds) AS queue_seconds,
			SUM(interference_seconds) AS interference_seconds, SUM(overhead_seconds) AS overhead_seconds
		FROM task_executions GROUP BY execution_run_id
	) t ON t.execution_run_id=e.id
	LEFT JOIN (
		SELECT execution_run_id, SUM(duration_seconds) AS transfer_seconds, SUM(bytes) AS transferred_bytes
		FROM data_transfer_observations GROUP BY execution_run_id
	) d ON d.execution_run_id=e.id
	UNION ALL
	SELECT s.id, '', 'interactive', 0,
		CASE s.status WHEN 'connected' THEN 'running' WHEN 'closed' THEN 'completed' WHEN 'failed' THEN 'failed' ELSE 'created' END,
		'', 0, 0, s.failure, 0, 0, 0, 0, 0, 0, 0, 0,
		'interactive', 'Interactive session', s.resource_id, s.runtime_id, s.connection_id, s.actor_id, '',
		COALESCE(s.connected_at, s.created_at), s.finished_at
	FROM console_sessions s
	UNION ALL
	SELECT c.id, '', 'real', 0, c.status, '', 0, 0, c.failure, 1,
		CASE WHEN c.status='completed' THEN 1 ELSE 0 END, 0, 0, 0, 0, 0, 0,
		'standalone', 'Standalone command', c.resource_id, c.runtime_id, c.connection_id, c.actor_id, c.command_text,
		c.started_at, c.finished_at
	FROM console_commands c`

func scanRunFeed(scanner interface{ Scan(...any) error }) (*domain.ExecutionRun, error) {
	var run domain.ExecutionRun
	var startedAt, finishedAt sql.NullTime
	err := scanner.Scan(&run.ID, &run.SchedulePlanID, &run.Mode, &run.Seed, &run.Status,
		&run.EnvironmentSnapshotID, &run.MakespanSeconds, &run.Cost, &run.FailureReason,
		&run.ActivityCount, &run.CompletedActivityCount, &run.Breakdown.ComputeSeconds,
		&run.Breakdown.TransferSeconds, &run.Breakdown.QueueSeconds,
		&run.Breakdown.InterferenceSeconds, &run.Breakdown.OverheadSeconds, &run.TransferredBytes,
		&run.Kind, &run.Title, &run.ResourceID, &run.RuntimeID, &run.ConnectionID, &run.ActorID,
		&run.Command, &startedAt, &finishedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		run.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}
	return &run, nil
}

func (r *Repository) ListTasks(ctx context.Context, runID string) ([]domain.TaskExecution, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id, t.execution_run_id, t.plan_assignment_id,
		t.activity_id, t.planned_resource_id, COALESCE(t.allocated_resource_id, ''), t.attempt,
		CASE WHEN t.status='running' AND h.id IS NOT NULL THEN 'failed' ELSE t.status END,
		t.ready_at, t.data_ready_at, t.queued_at, t.started_at,
		CASE WHEN t.status='running' AND h.id IS NOT NULL THEN h.finished_at ELSE t.finished_at END,
		t.runtime_seconds, t.queue_seconds, t.transfer_seconds, t.transfer_bytes, t.interference_seconds,
		t.overhead_seconds, t.cost,
		CASE WHEN t.status='running' AND h.id IS NOT NULL THEN h.failure ELSE t.failure_reason END
		FROM task_executions t
		LEFT JOIN activity_handles h ON h.execution_run_id=t.execution_run_id
			AND h.activity_id=t.activity_id AND h.status IN ('failed', 'stopped')
		WHERE t.execution_run_id=? ORDER BY t.started_at, t.activity_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []domain.TaskExecution
	for rows.Next() {
		var task domain.TaskExecution
		if err := rows.Scan(&task.ID, &task.ExecutionRunID, &task.PlanAssignmentID,
			&task.ActivityID, &task.PlannedResourceID, &task.AllocatedResourceID,
			&task.Attempt, &task.Status, &task.ReadyAt, &task.DataReadyAt,
			&task.QueuedAt, &task.StartedAt, &task.FinishedAt, &task.RuntimeSeconds,
			&task.QueueSeconds, &task.TransferSeconds, &task.TransferBytes, &task.InterferenceSeconds,
			&task.OverheadSeconds, &task.Cost, &task.FailureReason); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *Repository) ListTransfers(ctx context.Context, runID string) ([]domain.DataTransfer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, execution_run_id,
		COALESCE(producer_activity_id, ''), COALESCE(consumer_activity_id, ''), source_resource_id,
		target_resource_id, bytes, started_at, finished_at, duration_seconds, cost
		FROM data_transfer_observations WHERE execution_run_id=?
		ORDER BY started_at, producer_activity_id, consumer_activity_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transfers := make([]domain.DataTransfer, 0)
	for rows.Next() {
		var transfer domain.DataTransfer
		if err := rows.Scan(&transfer.ID, &transfer.ExecutionRunID,
			&transfer.ProducerActivityID, &transfer.ConsumerActivityID,
			&transfer.SourceResourceID, &transfer.TargetResourceID, &transfer.Bytes,
			&transfer.StartedAt, &transfer.FinishedAt, &transfer.DurationSeconds,
			&transfer.Cost); err != nil {
			return nil, err
		}
		transfers = append(transfers, transfer)
	}
	return transfers, rows.Err()
}

func (r *Repository) ListEvents(ctx context.Context, runID string) ([]domainevents.Event, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM domain_events
		WHERE (aggregate_type='execution_run' AND aggregate_id=?)
		OR (aggregate_type='activity_execution' AND json_extract(payload, '$.payload.executionRunId')=?)
		ORDER BY occurred_at, rowid`, runID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domainevents.Event
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var event domainevents.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) Save(ctx context.Context, handle domain.ActivityHandle) error {
	endpoints, err := json.Marshal(handle.Endpoints)
	if err != nil {
		return fmt.Errorf("marshal handle endpoints: %w", err)
	}
	metadata, err := json.Marshal(handle.Metadata)
	if err != nil {
		return fmt.Errorf("marshal handle metadata: %w", err)
	}
	artifacts, err := json.Marshal(handle.Artifacts)
	if err != nil {
		return fmt.Errorf("marshal handle artifacts: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO activity_handles (
		id, execution_run_id, activity_id, resource_id, runtime_id, external_id,
		status, endpoints, started_at, finished_at, exit_code, failure, log, artifacts, metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET external_id=excluded.external_id,
		status=excluded.status, endpoints=excluded.endpoints,
		started_at=excluded.started_at, finished_at=excluded.finished_at,
		exit_code=excluded.exit_code, failure=excluded.failure, log=excluded.log,
		artifacts=excluded.artifacts, metadata=excluded.metadata, updated_at=CURRENT_TIMESTAMP`,
		handle.ID, handle.RunID, handle.ActivityID, handle.ResourceID, handle.RuntimeID,
		handle.ExternalID, handle.Status, string(endpoints), handle.StartedAt,
		handle.FinishedAt, handle.ExitCode, handle.Failure, handle.Log, string(artifacts), string(metadata))
	return err
}

func (r *Repository) Find(ctx context.Context, id string) (*domain.ActivityHandle, error) {
	handle, err := scanHandle(r.db.QueryRowContext(ctx, `SELECT id, execution_run_id, activity_id,
		resource_id, runtime_id, external_id, status, endpoints, started_at,
		finished_at, exit_code, failure, log, artifacts, metadata FROM activity_handles WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return handle, err
}

func (r *Repository) ListHandles(ctx context.Context, runID string) ([]domain.ActivityHandle, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, execution_run_id, activity_id,
		resource_id, runtime_id, external_id, status, endpoints, started_at,
		finished_at, exit_code, failure, log, artifacts, metadata FROM activity_handles
		WHERE execution_run_id=? ORDER BY activity_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var handles []domain.ActivityHandle
	for rows.Next() {
		handle, err := scanHandle(rows)
		if err != nil {
			return nil, err
		}
		handles = append(handles, *handle)
	}
	return handles, rows.Err()
}

func scanHandle(scanner interface{ Scan(...any) error }) (*domain.ActivityHandle, error) {
	var handle domain.ActivityHandle
	var endpoints, artifacts, metadata string
	if err := scanner.Scan(&handle.ID, &handle.RunID, &handle.ActivityID, &handle.ResourceID,
		&handle.RuntimeID, &handle.ExternalID, &handle.Status, &endpoints,
		&handle.StartedAt, &handle.FinishedAt, &handle.ExitCode, &handle.Failure, &handle.Log,
		&artifacts, &metadata); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(endpoints), &handle.Endpoints); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadata), &handle.Metadata); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(artifacts), &handle.Artifacts); err != nil {
		return nil, err
	}
	return &handle, nil
}

func (r *Repository) SaveTask(ctx context.Context, task domain.TaskExecution) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	previous, err := taskStatus(ctx, tx, task.ExecutionRunID, task.ActivityID, task.Attempt)
	if err != nil {
		return err
	}
	if err := saveTask(ctx, tx, task); err != nil {
		return err
	}
	if previous != string(task.Status) {
		if err := appendTaskEvent(ctx, tx, task); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) CompleteRun(ctx context.Context, trace domain.ExecutionTrace) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, task := range trace.Tasks {
		var previous string
		if trace.Mode != domain.ExecutionModeSimulation {
			previous, err = taskStatus(ctx, tx, task.ExecutionRunID, task.ActivityID, task.Attempt)
			if err != nil {
				return err
			}
		}
		if err := saveTask(ctx, tx, task); err != nil {
			return err
		}
		if trace.Mode != domain.ExecutionModeSimulation && previous != string(task.Status) {
			if err := appendTaskEvent(ctx, tx, task); err != nil {
				return err
			}
		}
	}
	for _, transfer := range trace.Transfers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO data_transfer_observations (
			id, execution_run_id, producer_activity_id, consumer_activity_id,
			source_resource_id, target_resource_id, bytes, status, started_at,
			finished_at, duration_seconds, cost
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'completed', ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET status='completed', started_at=excluded.started_at,
			finished_at=excluded.finished_at, duration_seconds=excluded.duration_seconds,
			cost=excluded.cost`, transfer.ID,
			trace.RunID, nullableString(transfer.ProducerActivityID), nullableString(transfer.ConsumerActivityID),
			transfer.SourceResourceID, transfer.TargetResourceID, transfer.Bytes,
			transfer.StartedAt, transfer.FinishedAt, transfer.DurationSeconds,
			transfer.Cost); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE execution_runs SET status='completed',
		finished_at=CURRENT_TIMESTAMP, makespan_seconds=?, cost=?, failure_reason=''
		WHERE id=?`, trace.Executed.MakespanSeconds, trace.Executed.Cost, trace.RunID)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return fmt.Errorf("execution run %q not found", trace.RunID)
	}
	if err := dbevents.Append(ctx, tx, domainevents.Event{
		Type: domainevents.ExecutionCompleted, AggregateType: "execution_run", AggregateID: trace.RunID,
		Payload: map[string]any{"makespanSeconds": trace.Executed.MakespanSeconds, "cost": trace.Executed.Cost},
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) FailRun(ctx context.Context, id, reason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE execution_runs SET status='failed',
		finished_at=CURRENT_TIMESTAMP, failure_reason=? WHERE id=?`, reason, id)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return fmt.Errorf("execution run %q not found", id)
	}
	if err := dbevents.Append(ctx, tx, domainevents.Event{
		Type: domainevents.ExecutionFailed, AggregateType: "execution_run", AggregateID: id,
		Payload: map[string]any{"reason": reason},
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func taskStatus(ctx context.Context, tx *sql.Tx, runID, activityID string, attempt int) (string, error) {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT status FROM task_executions
		WHERE execution_run_id=? AND activity_id=? AND attempt=?`, runID, activityID, attempt).Scan(&status)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return status, err
}

func appendTaskEvent(ctx context.Context, tx *sql.Tx, task domain.TaskExecution) error {
	eventType := ""
	switch task.Status {
	case domain.TaskRunning:
		eventType = domainevents.ActivityStarted
	case domain.TaskCompleted:
		eventType = domainevents.ActivityCompleted
	case domain.TaskFailed, domain.TaskCancelled:
		eventType = domainevents.ActivityFailed
	default:
		return nil
	}
	lifecycleID := fmt.Sprintf("%s:%s:%d:%s", task.ExecutionRunID, task.ActivityID, task.Attempt, task.Status)
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO activity_lifecycle_events (
		id, task_execution_id, phase, status, started_at, finished_at,
		duration_seconds, source, error, metadata
	) VALUES (?, ?, 'execution', ?, ?, ?, ?, 'measured', ?, '{}')`, lifecycleID,
		task.ID, task.Status, task.StartedAt, task.FinishedAt, task.RuntimeSeconds,
		task.FailureReason); err != nil {
		return err
	}
	return dbevents.Append(ctx, tx, domainevents.Event{
		Type: eventType, AggregateType: "activity_execution", AggregateID: task.ID,
		OccurredAt: time.Now().UTC(), Payload: map[string]any{
			"executionRunId": task.ExecutionRunID, "activityId": task.ActivityID,
			"status": task.Status, "runtimeSeconds": task.RuntimeSeconds,
			"failureReason": task.FailureReason,
		},
	})
}

func saveTask(ctx context.Context, tx *sql.Tx, task domain.TaskExecution) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO task_executions (
		id, execution_run_id, plan_assignment_id, activity_id, planned_resource_id,
		allocated_resource_id, attempt, status, ready_at, data_ready_at, queued_at,
		started_at, finished_at, runtime_seconds, queue_seconds, transfer_seconds, transfer_bytes,
		interference_seconds, overhead_seconds, cost, failure_reason
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(execution_run_id, activity_id, attempt) DO UPDATE SET
		allocated_resource_id=excluded.allocated_resource_id, status=excluded.status,
		started_at=excluded.started_at, finished_at=excluded.finished_at,
		runtime_seconds=excluded.runtime_seconds, queue_seconds=excluded.queue_seconds,
		transfer_seconds=excluded.transfer_seconds, transfer_bytes=excluded.transfer_bytes,
		interference_seconds=excluded.interference_seconds,
		overhead_seconds=excluded.overhead_seconds, cost=excluded.cost,
		failure_reason=excluded.failure_reason`,
		task.ID, task.ExecutionRunID, task.PlanAssignmentID, task.ActivityID,
		task.PlannedResourceID, nullableString(task.AllocatedResourceID), task.Attempt,
		task.Status, task.ReadyAt, task.DataReadyAt, task.QueuedAt, task.StartedAt,
		task.FinishedAt, task.RuntimeSeconds, task.QueueSeconds, task.TransferSeconds, task.TransferBytes,
		task.InterferenceSeconds, task.OverheadSeconds, task.Cost, task.FailureReason)
	return err
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
