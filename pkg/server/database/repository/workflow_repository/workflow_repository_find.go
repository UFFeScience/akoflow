package workflow_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

func (w *WorkflowRepository) Find(workflowId int) (workflow_entity.Workflow, error) {

	database := repository.Database{}
	c := database.Connect()

	row := c.QueryRow("SELECT id, namespace, name, raw_workflow, status FROM "+w.tableName+" WHERE id = ?", workflowId)

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
