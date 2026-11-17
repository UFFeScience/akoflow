package workflow

import (
	"context"
	"database/sql"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) SetArchived(ctx context.Context, id string, archived bool) error {
	status := "published"
	if archived {
		status = "archived"
	}
	result, err := r.db.ExecContext(ctx, `UPDATE workflow_versions SET status=? WHERE id=(SELECT id FROM workflow_versions WHERE workflow_id=? ORDER BY version DESC LIMIT 1)`, status, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) versionHeader(ctx context.Context, id string) (domain.WorkflowVersion, error) {
	var workflow domain.WorkflowVersion
	err := r.db.QueryRowContext(ctx, `SELECT id, workflow_id, version, definition_hash,
        (SELECT status='archived' FROM workflow_versions latest WHERE latest.workflow_id=workflow_versions.workflow_id ORDER BY version DESC LIMIT 1)
        FROM workflow_versions WHERE id=?`, id).Scan(&workflow.ID, &workflow.WorkflowID,
		&workflow.Version, &workflow.DefinitionHash, &workflow.Archived)
	return workflow, err
}
