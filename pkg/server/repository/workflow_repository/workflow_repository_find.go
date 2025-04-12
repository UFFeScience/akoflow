package workflow_repository

import (
	"fmt"
	"strconv"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *WorkflowRepository) Find(workflowId int) (workflow_entity.Workflow, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, namespace, name, raw_workflow, status FROM %s WHERE id = %d",
		w.tableName,
		workflowId,
	)

	resp, err := db.Query(query)
	if err != nil {
		return workflow_entity.Workflow{}, err
	}

	results, ok := resp["results"].([]interface{})
	if !ok || len(results) == 0 {
		return workflow_entity.Workflow{}, fmt.Errorf("invalid response format")
	}

	rows := results[0].(map[string]interface{})["rows"].([]interface{})
	if len(rows) == 0 {
		return workflow_entity.Workflow{}, fmt.Errorf("workflow not found")
	}

	columns := results[0].(map[string]interface{})["columns"].([]interface{})
	values := rows[0].([]interface{})

	columnMap := map[string]interface{}{}
	for i, col := range columns {
		columnMap[col.(string)] = values[i]
	}

	result := workflow_entity.WorkflowDatabase{
		ID:          int(columnMap["id"].(float64)),
		Namespace:   columnMap["namespace"].(string),
		Name:        columnMap["name"].(string),
		RawWorkflow: columnMap["raw_workflow"].(string),
		Status: func() int {
			statusStr, ok := columnMap["status"].(string)
			if !ok {
				return 0 // Default value or handle error appropriately
			}
			statusInt, err := strconv.Atoi(statusStr)
			if err != nil {
				return 0 // Default value or handle error appropriately
			}
			return statusInt
		}(),
	}

	wf := workflow_entity.DatabaseToWorkflow(workflow_entity.ParamsDatabaseToWorkflow{
		WorkflowDatabase: result,
	})

	return wf, nil
}
