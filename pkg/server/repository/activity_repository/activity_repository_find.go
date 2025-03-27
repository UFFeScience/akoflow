package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) Find(id int) (workflow_activity_entity.WorkflowActivities, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, workflow_id, namespace, name, image, resource_k8s_base64, status, proc_id FROM "+w.tableNameActivity+" WHERE id = ?", id)
	if err != nil {
		return workflow_activity_entity.WorkflowActivities{}, err
	}

	var wfaDatabase workflow_activity_entity.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status, &wfaDatabase.ProcId)
		if err != nil {
			return workflow_activity_entity.WorkflowActivities{}, err
		}
	}

	activity := workflow_activity_entity.DatabaseToWorkflowActivities(workflow_activity_entity.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})

	err = c.Close()
	if err != nil {
		return workflow_activity_entity.WorkflowActivities{}, err
	}

	return activity, nil
}

func (w *ActivityRepository) GetPreactivitiesCompleted() ([]workflow_activity_entity.WorkflowPreActivityDatabase, error) {

	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, activity_id, workflow_id, namespace, name, resource_k8s_base64, status, log FROM " + w.tableNamePreActivity + " WHERE status = 2")

	if err != nil {
		return nil, err
	}

	var preActivities []workflow_activity_entity.WorkflowPreActivityDatabase

	for rows.Next() {
		var preActivity workflow_activity_entity.WorkflowPreActivityDatabase
		err = rows.Scan(&preActivity.Id, &preActivity.ActivityId, &preActivity.WorkflowId, &preActivity.Namespace, &preActivity.Name, &preActivity.ResourceK8sBase64, &preActivity.Status, &preActivity.Log)
		if err != nil {
			return nil, err
		}
		preActivities = append(preActivities, preActivity)
	}

	err = c.Close()

	if err != nil {
		return nil, err
	}

	return preActivities, nil

}
