package workflow_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *WorkflowRepository) GetPendingWorkflows(namespace string) ([]workflow_entity.Workflow, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, namespace, runtime, name, raw_workflow, status FROM %s WHERE namespace = '%s' AND status IN (%d, %d)",
		w.tableName,
		namespace,
		StatusRunning,
		StatusCreated,
	)

	resp, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	results, ok := resp["results"].([]interface{})
	if !ok || len(results) == 0 {
		return nil, fmt.Errorf("invalid response from rqlite")
	}

	resultMap := results[0].(map[string]interface{})
	columns := resultMap["columns"].([]interface{})
	rows, ok := resultMap["rows"].([]interface{})
	if !ok || len(rows) == 0 {
		return nil, nil
	}

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
			Runtime:     data["runtime"].(string),
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
