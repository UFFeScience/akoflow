package activity_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) FindPreActivity(activityID int) (workflow_activity_entity.WorkflowPreActivityDatabase, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, activity_id, workflow_id, namespace, name, resource_k8s_base64, status, log FROM %s WHERE activity_id = %d",
		w.tableNamePreActivity,
		activityID,
	)

	resp, err := db.Query(query)
	if err != nil {
		return workflow_activity_entity.WorkflowPreActivityDatabase{}, err
	}

	results := resp["results"].([]interface{})
	if len(results) == 0 {
		return workflow_activity_entity.WorkflowPreActivityDatabase{}, fmt.Errorf("invalid rqlite response")
	}

	rows := results[0].(map[string]interface{})["rows"].([]interface{})
	if len(rows) == 0 {
		return workflow_activity_entity.WorkflowPreActivityDatabase{}, fmt.Errorf("pre-activity not found")
	}

	columns := results[0].(map[string]interface{})["columns"].([]interface{})
	row := rows[0].([]interface{})

	data := map[string]interface{}{}
	for i, col := range columns {
		data[col.(string)] = row[i]
	}

	preActivity := workflow_activity_entity.WorkflowPreActivityDatabase{
		Id:                int(data["id"].(float64)),
		ActivityId:        int(data["activity_id"].(float64)),
		WorkflowId:        int(data["workflow_id"].(float64)),
		Namespace:         data["namespace"].(string),
		Name:              data["name"].(string),
		ResourceK8sBase64: toNullableString(data["resource_k8s_base64"]),
		Status:            int(data["status"].(float64)),
		Log:               toNullableString(data["log"]),
	}

	return preActivity, nil
}
