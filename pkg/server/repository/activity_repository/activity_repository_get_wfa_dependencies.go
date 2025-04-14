package activity_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) GetWfaDependencies(workflowId int) ([]workflow_activity_entity.WorkflowActivityDependencyDatabase, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, workflow_id, activity_id, depend_on_activity FROM %s WHERE workflow_id = %d",
		w.tableNameActivityDependencies,
		workflowId,
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
	rows := resultMap["rows"].([]interface{})

	var dependencies []workflow_activity_entity.WorkflowActivityDependencyDatabase
	for _, rowData := range rows {
		row := rowData.([]interface{})
		data := map[string]interface{}{}
		for i, col := range columns {
			data[col.(string)] = row[i]
		}

		dep := workflow_activity_entity.WorkflowActivityDependencyDatabase{
			Id:          int(data["id"].(float64)),
			WorkflowId:  int(data["workflow_id"].(float64)),
			ActivityId:  int(data["activity_id"].(float64)),
			DependsOnId: int(data["depend_on_activity"].(float64)),
		}

		dependencies = append(dependencies, dep)
	}

	return dependencies, nil
}
