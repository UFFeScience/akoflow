package workflow_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *WorkflowRepository) Find(workflowId int) (workflow_entity.Workflow, error) {

	database := repository.Database{}
	c := database.Connect()

	row := c.QueryRow("SELECT * FROM "+w.tableName+" WHERE ID = ?", workflowId)

	result := workflow_entity.WorkflowDatabase{}
	err := row.Scan(&result.ID, &result.Namespace, &result.Name, &result.RawWorkflow, &result.Status)

	wf := workflow_entity.DatabaseToWorkflow(workflow_entity.ParamsDatabaseToWorkflow{WorkflowDatabase: result})

	if err != nil {
		return workflow_entity.Workflow{}, err
	}

	err = c.Close()
	if err != nil {
		return workflow_entity.Workflow{}, err
	}

	return wf, nil

}
