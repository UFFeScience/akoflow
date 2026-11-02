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

func (r *Repository) ListHeaders(ctx context.Context) ([]domain.WorkflowDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT w.id,w.external_id,w.name,w.namespace,v.id,v.version,v.definition_hash
		FROM workflow_definitions w JOIN (`+latestVersions+`) v ON v.workflow_id=w.id ORDER BY w.namespace,w.name,w.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.WorkflowDefinition, 0)
	for rows.Next() {
		var item domain.WorkflowDefinition
		if err := rows.Scan(&item.ID, &item.ExternalID, &item.Name, &item.Namespace, &item.Version.ID,
			&item.Version.Version, &item.Version.DefinitionHash); err != nil {
			return nil, err
		}
		item.Version.WorkflowID = item.ID
		items = append(items, item)
	}
	return items, rows.Err()
}
