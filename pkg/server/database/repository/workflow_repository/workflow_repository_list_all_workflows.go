package workflow_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type ListAllWorkflowParams struct {
	All     bool
	Page    *int
	PerPage *int
}

func (w *WorkflowRepository) ListAllWorkflows(params *ListAllWorkflowParams) ([]workflow_entity.Workflow, error) {

	database := repository.Database{}
	c := database.Connect()

	if params == nil {
		params = &ListAllWorkflowParams{All: true}
	}

	query := "SELECT id, namespace, name, raw_workflow, status FROM " + w.tableName + " ORDER BY id DESC"

	if !params.All {
		query += " LIMIT " + string(rune(*params.PerPage)) + " OFFSET " + string(rune(*params.Page**params.PerPage)) + ";"
	}

	rows, err := c.Query(query)
	if err != nil {
		return nil, err
	}

	var workflows []workflow_entity.Workflow

	for rows.Next() {
		result := workflow_entity.WorkflowDatabase{}
		err = rows.Scan(&result.ID, &result.Namespace, &result.Name, &result.RawWorkflow, &result.Status)
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
