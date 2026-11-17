package workflow

import (
	"context"
	"database/sql"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) FindHeader(ctx context.Context, id string) (*domain.WorkflowDefinition, error) {
	var definition domain.WorkflowDefinition
	err := r.db.QueryRowContext(ctx, `SELECT id,external_id,name,namespace,
      COALESCE((SELECT status='archived' FROM workflow_versions WHERE workflow_id=workflow_definitions.id ORDER BY version DESC LIMIT 1),0)
      FROM workflow_definitions WHERE id=?`, id).
		Scan(&definition.ID, &definition.ExternalID, &definition.Name, &definition.Namespace, &definition.Archived)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &definition, err
}

func (r *Repository) ListHeaders(ctx context.Context) ([]domain.WorkflowDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT w.id,w.external_id,w.name,w.namespace,v.id,v.version,v.definition_hash,v.status='archived'
		FROM workflow_definitions w JOIN (`+latestVersions+`) v ON v.workflow_id=w.id ORDER BY w.namespace,w.name,w.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.WorkflowDefinition, 0)
	for rows.Next() {
		var item domain.WorkflowDefinition
		if err := rows.Scan(&item.ID, &item.ExternalID, &item.Name, &item.Namespace, &item.Version.ID,
			&item.Version.Version, &item.Version.DefinitionHash, &item.Archived); err != nil {
			return nil, err
		}
		item.Version.Archived = item.Archived
		item.Version.WorkflowID = item.ID
		items = append(items, item)
	}
	return items, rows.Err()
}
