package workflow

import (
	"context"
	"database/sql"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) FindHeader(ctx context.Context, id string) (*domain.WorkflowDefinition, error) {
	var definition domain.WorkflowDefinition
	err := r.db.QueryRowContext(ctx, `SELECT id,external_id,name,namespace FROM workflow_definitions WHERE id=?`, id).
		Scan(&definition.ID, &definition.ExternalID, &definition.Name, &definition.Namespace)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &definition, err
}
