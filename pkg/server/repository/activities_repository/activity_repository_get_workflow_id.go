package activities_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
)

func (w *ActivityRepository) GetByWorkflowId(id int) ([]workflow.WorkflowActivities, error) {
	database := connector.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableNameActivity+" WHERE workflow_id = ?", id)
	if err != nil {
		return []workflow.WorkflowActivities{}, err
	}

	var activities []workflow.WorkflowActivities
	var wfaDatabase workflow.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
		if err != nil {
			return []workflow.WorkflowActivities{}, err
		}

		activity := workflow.DatabaseToWorkflowActivities(workflow.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})
		activities = append(activities, activity)

	}

	err = c.Close()
	if err != nil {
		return []workflow.WorkflowActivities{}, err
	}

	return activities, nil

}
