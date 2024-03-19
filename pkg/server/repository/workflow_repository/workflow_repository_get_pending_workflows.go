package workflow_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

func (w *WorkflowRepository) GetPendingWorkflows(namespace string) ([]workflow.Workflow, error) {

	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableName+" WHERE namespace = ? AND status IN (?, ?)", namespace, StatusRunning, StatusCreated)
	if err != nil {
		return nil, err
	}

	var workflows []workflow.Workflow

	for rows.Next() {
		result := workflow.WorkflowDatabase{}
		err = rows.Scan(&result.ID, &result.Namespace, &result.Name, &result.RawWorkflow, &result.Status)
		if err != nil {
			return nil, err
		}

		wf := workflow.DatabaseToWorkflow(workflow.ParamsDatabaseToWorkflow{WorkflowDatabase: result})
		workflows = append(workflows, wf)
	}

	err = c.Close()
	if err != nil {
		return nil, err
	}

	return workflows, nil
}
