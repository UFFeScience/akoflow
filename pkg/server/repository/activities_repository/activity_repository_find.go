package activities_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
)

func (w *ActivityRepository) Find(id int) (workflow.WorkflowActivities, error) {
	database := connector.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableNameActivity+" WHERE ID = ?", id)
	if err != nil {
		return workflow.WorkflowActivities{}, err
	}

	var wfaDatabase workflow.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
		if err != nil {
			return workflow.WorkflowActivities{}, err
		}
	}

	activity := workflow.DatabaseToWorkflowActivities(workflow.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})

	err = c.Close()
	if err != nil {
		return workflow.WorkflowActivities{}, err
	}

	return activity, nil
}
