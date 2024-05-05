package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) GetByWorkflowId(id int) ([]workflow_activity_entity.WorkflowActivities, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableNameActivity+" WHERE workflow_id = ?", id)
	if err != nil {
		return []workflow_activity_entity.WorkflowActivities{}, err
	}

	var activities []workflow_activity_entity.WorkflowActivities
	var wfaDatabase workflow_activity_entity.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
		if err != nil {
			return []workflow_activity_entity.WorkflowActivities{}, err
		}

		activity := workflow_activity_entity.DatabaseToWorkflowActivities(workflow_activity_entity.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})
		activities = append(activities, activity)

	}

	err = c.Close()
	if err != nil {
		return []workflow_activity_entity.WorkflowActivities{}, err
	}

	return activities, nil

}
