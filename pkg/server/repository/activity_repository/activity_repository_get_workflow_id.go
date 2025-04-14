package activity_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) GetByWorkflowId(id int) ([]workflow_activity_entity.WorkflowActivities, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, workflow_id, namespace, name, image, resource_k8s_base64, status FROM %s WHERE workflow_id = %d",
		w.tableNameActivity,
		id,
	)

	resp, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	results := resp["results"].([]interface{})
	if len(results) == 0 {
		return nil, fmt.Errorf("no results from rqlite")
	}

	resultMap := results[0].(map[string]interface{})
	columns := resultMap["columns"].([]interface{})
	rows := resultMap["rows"].([]interface{})

	var activities []workflow_activity_entity.WorkflowActivities
	for _, rowData := range rows {
		row := rowData.([]interface{})
		data := map[string]interface{}{}
		for i, col := range columns {
			data[col.(string)] = row[i]
		}

		activity := workflow_activity_entity.WorkflowActivityDatabase{
			Id:                int(data["id"].(float64)),
			WorkflowId:        int(data["workflow_id"].(float64)),
			Namespace:         data["namespace"].(string),
			Name:              data["name"].(string),
			Image:             data["image"].(string),
			ResourceK8sBase64: data["resource_k8s_base64"].(string),
			Status:            int(data["status"].(float64)),
		}

		wf := workflow_activity_entity.DatabaseToWorkflowActivities(
			workflow_activity_entity.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: activity},
		)

		activities = append(activities, wf)
	}

	return activities, nil
}
