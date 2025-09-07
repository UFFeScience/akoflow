package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
)

func (w *ActivityRepository) GetByWorkflowId(id int) ([]workflow_activity_entity.WorkflowActivities, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, workflow_id, namespace, name, image, runtime, resource_k8s_base64, status FROM "+w.tableNameActivity+" WHERE workflow_id = ?", id)
	if err != nil {
		return []workflow_activity_entity.WorkflowActivities{}, err
	}

	var activities []workflow_activity_entity.WorkflowActivities
	var wfaDatabase workflow_activity_entity.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.Runtime, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
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

func (w *ActivityRepository) GetAllRunningActivities() ([]workflow_activity_entity.WorkflowActivities, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, workflow_id, namespace, name, image, runtime, resource_k8s_base64, status FROM "+w.tableNameActivity+" WHERE status = ?", StatusRunning)
	if err != nil {
		return []workflow_activity_entity.WorkflowActivities{}, err
	}

	var activities []workflow_activity_entity.WorkflowActivities
	var wfaDatabase workflow_activity_entity.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.Runtime, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
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
