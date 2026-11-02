package data

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) ListGeneratedDataPage(ctx context.Context, page, pageSize int) (domain.GeneratedDataPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	result := domain.GeneratedDataPage{Page: page, PageSize: pageSize, Items: make([]domain.GeneratedDataItem, 0)}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM data_object_instances WHERE execution_run_id IS NOT NULL`).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := r.db.QueryContext(ctx, `WITH selected AS MATERIALIZED (
		SELECT id,execution_run_id,data_object_id,relative_path,size_bytes,created_at FROM data_object_instances
		WHERE execution_run_id IS NOT NULL ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?
	) SELECT i.id,i.execution_run_id,COALESCE(o.logical_name,''),i.relative_path,i.size_bytes,
		(SELECT COUNT(*) FROM data_locations l WHERE l.data_object_instance_id=i.id)
	FROM selected i LEFT JOIN data_objects o ON o.id=i.data_object_id
	ORDER BY i.created_at DESC,i.id DESC`, pageSize, (page-1)*pageSize)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.GeneratedDataItem
		if err := rows.Scan(&item.ID, &item.RunID, &item.LogicalName, &item.RelativePath, &item.SizeBytes, &item.Locations); err != nil {
			return result, err
		}
		item.Key = item.RunID + "-" + item.ID
		result.Items = append(result.Items, item)
	}
	result.HasNext = page*pageSize < result.Total
	return result, rows.Err()
}
