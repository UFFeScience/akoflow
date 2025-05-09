package workflow_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

func (w *WorkflowRepository) GetPendingWorkflows(namespace string) ([]workflow_entity.Workflow, error) {

	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, namespace, runtime, name, raw_workflow, status FROM "+w.tableName+" WHERE namespace = ? AND status IN (?, ?)", namespace, StatusRunning, StatusCreated)
	if err != nil {
		return nil, err
	}

	var workflows []workflow_entity.Workflow

	for rows.Next() {
		result := workflow_entity.WorkflowDatabase{}
		err = rows.Scan(&result.ID, &result.Namespace, &result.Runtime, &result.Name, &result.RawWorkflow, &result.Status)
		if err != nil {
			return nil, err
		}

		wf := workflow_entity.DatabaseToWorkflow(workflow_entity.ParamsDatabaseToWorkflow{WorkflowDatabase: result})
		workflows = append(workflows, wf)
	}

	err = c.Close()
	if err != nil {
		return nil, err
	}

	return workflows, nil
}
