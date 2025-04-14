package workflow_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type ListAllWorkflowParams struct {
	All     bool
	Page    *int
	PerPage *int
}

func (w *WorkflowRepository) ListAllWorkflows(params *ListAllWorkflowParams) ([]workflow_entity.Workflow, error) {
	db := repository.GetInstance()

	if params == nil {
		params = &ListAllWorkflowParams{All: true}
	}

	query := fmt.Sprintf("SELECT id, namespace, name, raw_workflow, status FROM %s ORDER BY id DESC", w.tableName)

	if !params.All && params.Page != nil && params.PerPage != nil {
		offset := (*params.Page) * (*params.PerPage)
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", *params.PerPage, offset)
	}

	resp, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	// Parse response
	results, ok := resp["results"].([]interface{})
	if !ok || len(results) == 0 {
		return nil, fmt.Errorf("invalid response from rqlite")
	}

	resultMap := results[0].(map[string]interface{})
	columns := resultMap["columns"].([]interface{})
	rows := resultMap["rows"].([]interface{})

	var workflows []workflow_entity.Workflow
	for _, rowData := range rows {
		row := rowData.([]interface{})
		data := map[string]interface{}{}
		for i, col := range columns {
			data[col.(string)] = row[i]
		}

		result := workflow_entity.WorkflowDatabase{
			ID:          int(data["id"].(float64)),
			Namespace:   data["namespace"].(string),
			Name:        data["name"].(string),
			RawWorkflow: data["raw_workflow"].(string),
			Status:      int(data["status"].(float64)),
		}

		wf := workflow_entity.DatabaseToWorkflow(workflow_entity.ParamsDatabaseToWorkflow{
			WorkflowDatabase: result,
		})

		workflows = append(workflows, wf)
	}

	return workflows, nil
}
