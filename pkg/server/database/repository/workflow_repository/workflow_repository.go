package workflow_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type WorkflowRepository struct {
	tableName string
}

var TableName = "workflows"
var Columns = "(id INTEGER PRIMARY KEY AUTOINCREMENT, namespace TEXT, runtime TEXT, name TEXT, raw_workflow TEXT, status INTEGER)"

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2

func New() IWorkflowRepository {

	database := repository.Database{}
	c := database.Connect()

	err := repository.CreateOrVerifyTable(c, model.Workflow{})

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
	ListAllWorkflows(params *ListAllWorkflowParams) ([]workflow_entity.Workflow, error)
}
