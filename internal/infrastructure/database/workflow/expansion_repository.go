package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) SaveExpansion(ctx context.Context, value domain.WorkflowExpansion) (*domain.WorkflowExpansion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	metadata, err := json.Marshal(value.Metadata)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO workflow_expansions
		(id, workflow_version_id, execution_run_id, source_activity_id, source_event_id, sequence, result_revision, status, failure_reason, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_version_id, source_event_id) DO NOTHING`,
		value.ID, value.WorkflowVersionID, value.ExecutionRunID, value.SourceActivityID,
		value.SourceEventID, value.Sequence, value.ResultRevision, value.Status,
		value.FailureReason, string(metadata))
	if err != nil {
		return nil, fmt.Errorf("save workflow expansion: %w", err)
	}
	inserted, _ := result.RowsAffected()
	if inserted == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.FindExpansionByEvent(ctx, value.WorkflowVersionID, value.SourceEventID)
	}
	for _, activity := range value.Activities {
		definition, err := json.Marshal(activity)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO workflow_expansion_activities
			(expansion_id, activity_id, definition) VALUES (?, ?, ?)`,
			value.ID, activity.ID, string(definition)); err != nil {
			return nil, err
		}
	}
	for _, dependency := range value.Dependencies {
		if _, err = tx.ExecContext(ctx, `INSERT INTO workflow_expansion_dependencies
			(expansion_id, activity_id, depends_on_activity_id, dependency_type) VALUES (?, ?, ?, ?)`,
			value.ID, dependency.ActivityID, dependency.DependsOnActivityID, dependency.Type); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &value, nil
}

func (r *Repository) FindExpansionByEvent(ctx context.Context, workflowVersionID, sourceEventID string) (*domain.WorkflowExpansion, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, workflow_version_id, execution_run_id,
		source_activity_id, source_event_id, sequence, result_revision, status,
		failure_reason, metadata, created_at FROM workflow_expansions
		WHERE workflow_version_id=? AND source_event_id=?`, workflowVersionID, sourceEventID)
	value, err := r.scanExpansion(ctx, row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

func (r *Repository) ListExpansions(ctx context.Context, workflowVersionID, executionRunID string) ([]domain.WorkflowExpansion, error) {
	query := `SELECT id FROM workflow_expansions WHERE workflow_version_id=?`
	args := []any{workflowVersionID}
	if executionRunID != "" {
		query += ` AND execution_run_id=?`
		args = append(args, executionRunID)
	}
	query += ` ORDER BY result_revision, id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	values := make([]domain.WorkflowExpansion, 0, len(ids))
	for _, id := range ids {
		value, err := r.findExpansion(ctx, id)
		if err != nil {
			return nil, err
		}
		values = append(values, *value)
	}
	return values, nil
}

func (r *Repository) findExpansion(ctx context.Context, id string) (*domain.WorkflowExpansion, error) {
	return r.scanExpansion(ctx, r.db.QueryRowContext(ctx, `SELECT id, workflow_version_id,
		execution_run_id, source_activity_id, source_event_id, sequence, result_revision,
		status, failure_reason, metadata, created_at FROM workflow_expansions WHERE id=?`, id))
}

type scanner interface{ Scan(...any) error }

func (r *Repository) scanExpansion(ctx context.Context, row scanner) (*domain.WorkflowExpansion, error) {
	var value domain.WorkflowExpansion
	var metadata string
	if err := row.Scan(
		&value.ID, &value.WorkflowVersionID, &value.ExecutionRunID,
		&value.SourceActivityID, &value.SourceEventID, &value.Sequence,
		&value.ResultRevision, &value.Status, &value.FailureReason, &metadata, &value.CreatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadata), &value.Metadata); err != nil {
		return nil, err
	}
	activities, err := r.db.QueryContext(ctx, `SELECT definition FROM workflow_expansion_activities WHERE expansion_id=? ORDER BY activity_id`, value.ID)
	if err != nil {
		return nil, err
	}
	for activities.Next() {
		var raw string
		var activity domain.Activity
		if err := activities.Scan(&raw); err != nil {
			activities.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &activity); err != nil {
			activities.Close()
			return nil, err
		}
		value.Activities = append(value.Activities, activity)
	}
	activities.Close()
	dependencies, err := r.db.QueryContext(ctx, `SELECT activity_id, depends_on_activity_id,
		dependency_type FROM workflow_expansion_dependencies WHERE expansion_id=?
		ORDER BY activity_id, depends_on_activity_id`, value.ID)
	if err != nil {
		return nil, err
	}
	defer dependencies.Close()
	for dependencies.Next() {
		var dependency domain.ActivityDependency
		if err := dependencies.Scan(&dependency.ActivityID, &dependency.DependsOnActivityID, &dependency.Type); err != nil {
			return nil, err
		}
		value.Dependencies = append(value.Dependencies, dependency)
	}
	return &value, dependencies.Err()
}
