package workflow_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

func (w *WorkflowRepository) Find(workflowId int) (workflow.Workflow, error) {

	database := repository.Database{}
	c := database.Connect()

	row := c.QueryRow("SELECT * FROM "+w.tableName+" WHERE ID = ?", workflowId)

	result := workflow.WorkflowDatabase{}
	err := row.Scan(&result.ID, &result.Namespace, &result.Name, &result.RawWorkflow, &result.Status)

	wf := workflow.DatabaseToWorkflow(workflow.ParamsDatabaseToWorkflow{WorkflowDatabase: result})

	if err != nil {
		return workflow.Workflow{}, err
	}

	err = c.Close()
	if err != nil {
		return workflow.Workflow{}, err
	}

	return wf, nil

}
