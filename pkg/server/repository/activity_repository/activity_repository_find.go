package activity_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) Find(id int) (workflow_activity_entity.WorkflowActivities, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, workflow_id, namespace, name, image, resource_k8s_base64, status, proc_id FROM %s WHERE id = %d",
		w.tableNameActivity,
		id,
	)

	resp, err := db.Query(query)
	if err != nil {
		return workflow_activity_entity.WorkflowActivities{}, err
	}

	results := resp["results"].([]interface{})
	if len(results) == 0 {
		return workflow_activity_entity.WorkflowActivities{}, fmt.Errorf("no result from rqlite")
	}

	rows := results[0].(map[string]interface{})["rows"].([]interface{})
	if len(rows) == 0 {
		return workflow_activity_entity.WorkflowActivities{}, fmt.Errorf("activity not found")
	}

	columns := results[0].(map[string]interface{})["columns"].([]interface{})
	row := rows[0].([]interface{})

	data := map[string]interface{}{}
	for i, col := range columns {
		data[col.(string)] = row[i]
	}

	wfa := workflow_activity_entity.WorkflowActivityDatabase{
		Id:                int(data["id"].(float64)),
		WorkflowId:        int(data["workflow_id"].(float64)),
		Namespace:         data["namespace"].(string),
		Name:              data["name"].(string),
		Image:             data["image"].(string),
		ResourceK8sBase64: data["resource_k8s_base64"].(string),
		Status:            int(data["status"].(float64)),
		ProcId:            toNullableString(data["proc_id"]),
	}

	return workflow_activity_entity.DatabaseToWorkflowActivities(
		workflow_activity_entity.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfa},
	), nil
}

func (w *ActivityRepository) GetPreactivitiesCompleted() ([]workflow_activity_entity.WorkflowPreActivityDatabase, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, activity_id, workflow_id, namespace, name, resource_k8s_base64, status, log FROM %s WHERE status = 2",
		w.tableNamePreActivity,
	)

	resp, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	results := resp["results"].([]interface{})
	if len(results) == 0 {
		return nil, fmt.Errorf("no result from rqlite")
	}

	rows := results[0].(map[string]interface{})["rows"].([]interface{})
	columns := results[0].(map[string]interface{})["columns"].([]interface{})

	var preActivities []workflow_activity_entity.WorkflowPreActivityDatabase

	for _, rowData := range rows {
		row := rowData.([]interface{})
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

		preActivities = append(preActivities, preActivity)
	}

	return preActivities, nil
}
