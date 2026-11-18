package execution

import (
	"context"
	"database/sql"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

const scheduleColumns = `s.id,s.name,s.workflow_version_id,s.schedule_plan_id,s.frequency,s.local_time,s.weekday,
	s.time_zone,s.missed_policy,s.overlap_policy,s.paused,s.next_at`

const scheduleSummary = `SELECT ` + scheduleColumns + `,
	last.scheduled_at,last.handled_at,last.outcome,last.detail,last.run_id,
	launch.scheduled_at,launch.handled_at,launch.run_id,run.status
	FROM execution_schedules s
	LEFT JOIN execution_schedule_occurrences last ON last.schedule_id=s.id AND last.scheduled_at=(
		SELECT MAX(o.scheduled_at) FROM execution_schedule_occurrences o WHERE o.schedule_id=s.id)
	LEFT JOIN execution_schedule_occurrences launch ON launch.schedule_id=s.id AND launch.scheduled_at=(
		SELECT MAX(o.scheduled_at) FROM execution_schedule_occurrences o WHERE o.schedule_id=s.id AND o.run_id IS NOT NULL)
	LEFT JOIN execution_runs run ON run.id=launch.run_id`

func (r *Repository) SaveSchedule(ctx context.Context, s domainexecution.Schedule) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO execution_schedules
		(id,name,workflow_version_id,schedule_plan_id,frequency,local_time,weekday,time_zone,missed_policy,overlap_policy,paused,next_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, s.ID, s.Name, s.WorkflowVersionID, s.PlanID, s.Frequency,
		s.LocalTime, s.Weekday, s.TimeZone, s.MissedPolicy, s.OverlapPolicy, s.Paused, s.NextAt.Unix())
	return err
}

func (r *Repository) ListSchedules(ctx context.Context) ([]domainexecution.Schedule, error) {
	rows, err := r.db.QueryContext(ctx, scheduleSummary+" ORDER BY s.name,s.id")
	if err != nil {
		return nil, err
	}
	return scanSchedules(rows, true)
}

func (r *Repository) FindSchedule(ctx context.Context, id string) (*domainexecution.Schedule, error) {
	s, err := scanSchedule(r.db.QueryRowContext(ctx, scheduleSummary+" WHERE s.id=?", id), true)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

func (r *Repository) DueSchedules(ctx context.Context, now time.Time) ([]domainexecution.Schedule, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+scheduleColumns+
		" FROM execution_schedules s WHERE paused=0 AND next_at<=? ORDER BY next_at,id", now.Unix())
	if err != nil {
		return nil, err
	}
	return scanSchedules(rows, false)
}

func (r *Repository) SetSchedulePaused(ctx context.Context, id string, paused bool, next time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE execution_schedules SET paused=?,next_at=CASE WHEN ? THEN next_at ELSE ? END
		WHERE id=? AND paused<>?`, paused, paused, next.Unix(), id, paused)
	return err
}

func scanSchedules(rows *sql.Rows, summary bool) ([]domainexecution.Schedule, error) {
	defer rows.Close()
	result := []domainexecution.Schedule{}
	for rows.Next() {
		s, err := scanSchedule(rows, summary)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func scanSchedule(row interface{ Scan(...any) error }, summary bool) (domainexecution.Schedule, error) {
	var s domainexecution.Schedule
	var next int64
	values := []any{&s.ID, &s.Name, &s.WorkflowVersionID, &s.PlanID, &s.Frequency, &s.LocalTime,
		&s.Weekday, &s.TimeZone, &s.MissedPolicy, &s.OverlapPolicy, &s.Paused, &next}
	var occurrenceAt, handledAt, launchedAt, launchHandled sql.NullInt64
	var outcome, detail, occurrenceRun, launchRun, status sql.NullString
	if summary {
		values = append(values, &occurrenceAt, &handledAt, &outcome, &detail, &occurrenceRun,
			&launchedAt, &launchHandled, &launchRun, &status)
	}
	if err := row.Scan(values...); err != nil {
		return s, err
	}
	s.NextAt = time.Unix(next, 0).UTC()
	if occurrenceAt.Valid {
		s.LastOccurrence = &domainexecution.ScheduleOutcome{ScheduledAt: scheduleTime(occurrenceAt), HandledAt: scheduleTime(handledAt),
			Outcome: outcome.String, Detail: detail.String, RunID: occurrenceRun.String}
	}
	if launchedAt.Valid {
		s.LastLaunch = &domainexecution.ScheduleOutcome{ScheduledAt: scheduleTime(launchedAt), HandledAt: scheduleTime(launchHandled),
			Outcome: "launched", RunID: launchRun.String, RunStatus: domainexecution.ExecutionRunStatus(status.String)}
	}
	return s, nil
}

func scheduleTime(value sql.NullInt64) time.Time {
	return time.Unix(value.Int64, 0).UTC()
}

func (r *Repository) RecordOccurrence(ctx context.Context, schedule domainexecution.Schedule, next time.Time,
	outcome domainexecution.ScheduleOutcome, run domain.ExecutionRun, job domainqueue.Job) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE execution_schedules SET next_at=? WHERE id=? AND paused=0 AND next_at=?",
		next.Unix(), schedule.ID, schedule.NextAt.Unix())
	if err != nil {
		return err
	}
	claimed, err := result.RowsAffected()
	if err != nil || claimed == 0 {
		return err
	}
	if outcome.Outcome == "launched" && schedule.OverlapPolicy == "skip" {
		var active bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM execution_schedule_occurrences o
			JOIN execution_runs r ON r.id=o.run_id WHERE o.schedule_id=? AND r.status IN ('created','running'))`, schedule.ID).Scan(&active)
		if err != nil {
			return err
		}
		if active {
			outcome.Outcome = "overlap"
		}
	}
	var runID any
	if outcome.Outcome == "launched" {
		if _, err := submitRunIn(ctx, tx, run, job); err != nil {
			return err
		}
		runID = run.ID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO execution_schedule_occurrences
		(schedule_id,scheduled_at,handled_at,outcome,run_id,detail) VALUES(?,?,?,?,?,?)`,
		schedule.ID, outcome.ScheduledAt.Unix(), outcome.HandledAt.Unix(), outcome.Outcome, runID, outcome.Detail); err != nil {
		return err
	}
	return tx.Commit()
}
