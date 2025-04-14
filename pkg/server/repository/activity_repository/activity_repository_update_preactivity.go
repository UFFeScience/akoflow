package activity_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) UpdatePreActivity(id int, preactivity workflow_activity_entity.WorkflowPreActivityDatabase) error {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"UPDATE %s SET status = %d, log = '%s', resource_k8s_base64 = '%s' WHERE activity_id = %d",
		w.tableNamePreActivity,
		preactivity.Status,
		preactivity.Log,
		preactivity.ResourceK8sBase64,
		id,
	)

	_, err := db.Exec(query)
	return err
}
