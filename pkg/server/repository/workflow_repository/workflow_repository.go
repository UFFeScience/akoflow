package workflow_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

type WorkflowRepository struct {
	tableName string
}

var TableName = "workflows"
var Columns = "(id INTEGER PRIMARY KEY AUTOINCREMENT, namespace TEXT, name TEXT, raw_workflow TEXT, status INTEGER)"

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2

func New() IWorkflowRepository {

	database := repository.Database{}
	c := database.Connect()

	err := repository.CreateOrVerifyTable(c, TableName, Columns)

	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &WorkflowRepository{tableName: TableName}
}

type IWorkflowRepository interface {
	Create(namespace string, workflow workflow.Workflow) (int, error)
	Find(workflowId int) (workflow.Workflow, error)
	GetPendingWorkflows(namespace string) ([]workflow.Workflow, error)
	UpdateStatus(id int, status int) error
}
