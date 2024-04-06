package workflow_repository

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/repository"
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
		println("Error creating table", err.Error())
		return nil
	}

	err = c.Close()
	if err != nil {
		println("Error closing connection", err.Error())
		return nil
	}

	return &WorkflowRepository{tableName: TableName}
}

type IWorkflowRepository interface {
	Create(namespace string, workflow workflow_entity.Workflow) (int, error)
	Find(workflowId int) (workflow_entity.Workflow, error)
	GetPendingWorkflows(namespace string) ([]workflow_entity.Workflow, error)
	UpdateStatus(id int, status int) error
}
