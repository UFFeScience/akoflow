package workflow_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

func (w *WorkflowRepository) Create(namespace string, workflow workflow_entity.Workflow) (int, error) {

	database := repository.Database{}
	c := database.Connect()

	rawWorkflow := workflow.GetBase64Workflow()

	result, err := c.Exec(
		"INSERT INTO "+w.tableName+" (namespace, runtime, name, raw_workflow, status) VALUES (?, ?, ?, ?, ?)",
		namespace,
		workflow.Spec.Runtime,
		workflow.Name,
		rawWorkflow,
		StatusCreated,
	)

	if err != nil {
		return 0, err
	}
	workflowId, _ := result.LastInsertId()

	err = c.Close()
	if err != nil {
		return 0, err
	}

	return int(workflowId), nil
}
