package workflow_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

type WorkflowRepository struct {
	tableName string
}

var TableName = "workflows"
var Columns = "(ID INTEGER PRIMARY KEY AUTOINCREMENT, namespace TEXT, name TEXT, raw_workflow TEXT, status INTEGER)"

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2

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

func (w *WorkflowRepository) Create(namespace string, workflow workflow.Workflow) (int, error) {

	database := connector.Database{}
	c := database.Connect()

	rawWorkflow := workflow.GetBase64Workflow()

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

func (w *WorkflowRepository) Find(workflowId int) (workflow.Workflow, error) {

	database := connector.Database{}
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

func (w *WorkflowRepository) GetPendingWorkflows(namespace string) ([]workflow.Workflow, error) {

	database := connector.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableName+" WHERE namespace = ? AND status = ?", namespace, StatusCreated)
	if err != nil {
		return nil, err
	}

	var workflows []workflow.Workflow

	for rows.Next() {
		result := workflow.WorkflowDatabase{}
		err = rows.Scan(&result.ID, &result.Namespace, &result.Name, &result.RawWorkflow, &result.Status)
		if err != nil {
			return nil, err
		}

		wf := workflow.DatabaseToWorkflow(workflow.ParamsDatabaseToWorkflow{WorkflowDatabase: result})
		workflows = append(workflows, wf)
	}

	err = c.Close()
	if err != nil {
		return nil, err
	}

	return workflows, nil
}
