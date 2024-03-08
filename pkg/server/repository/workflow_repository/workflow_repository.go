package workflow_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/jobs_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/workflow"
)

type WorkflowRepository struct {
	tableName string
}

var TableName = "workflows"
var Columns = "(ID INTEGER PRIMARY KEY AUTOINCREMENT, namespace TEXT, name TEXT, raw_workflow TEXT, status INTEGER)"

func New() *WorkflowRepository {

	database := connector.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, TableName, Columns)
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &WorkflowRepository{
		tableName: TableName,
	}
}

func (w *WorkflowRepository) Create(namespace string, workflow workflow.Workflow) error {

	database := connector.Database{}
	c := database.Connect()

	rawWorkflow := workflow.GetBase64Workflow()

	result, err := c.Exec("INSERT INTO "+w.tableName+" (namespace, name, raw_workflow, status) VALUES (?, ?, ?, ?)", namespace, workflow.Name, rawWorkflow, 0)

	if err != nil {
		return err
	}
	workflowId, _ := result.LastInsertId()

	jobsRepository := jobs_repository.New()
	err = jobsRepository.Create(namespace, int(workflowId), workflow.Spec.Activities)

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}
