package workflow_repository

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/repository"
)

func (w *WorkflowRepository) Create(namespace string, workflow workflow.Workflow) (int, error) {

	database := repository.Database{}
	c := database.Connect()

	rawWorkflow := workflow.GetBase64Workflow()

	resources := workflow.MakeResourcesK8s()

	println("Resources: ", resources)

	result, err := c.Exec("INSERT INTO "+w.tableName+" (namespace, name, raw_workflow, status) VALUES (?, ?, ?, ?)", namespace, workflow.Name, rawWorkflow, StatusCreated)

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
