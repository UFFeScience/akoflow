package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) FindPreActivity(activityID int) (workflow_activity_entity.WorkflowPreActivityDatabase, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, activity_id, workflow_id, namespace, name, resource_k8s_base64, status, log FROM "+w.tableNamePreActivity+" WHERE activity_id = ?", activityID)
	if err != nil {
		return workflow_activity_entity.WorkflowPreActivityDatabase{}, err
	}

	var wfaDatabase workflow_activity_entity.WorkflowPreActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.ActivityId, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status, &wfaDatabase.Log)
		if err != nil {
			return workflow_activity_entity.WorkflowPreActivityDatabase{}, err
		}
	}

	err = c.Close()
	if err != nil {
		return workflow_activity_entity.WorkflowPreActivityDatabase{}, err
	}

	return wfaDatabase, nil
}
