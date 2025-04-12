package activity_repository

import (
	"fmt"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type ResultGetActivitiesByWorkflowIds map[int][]workflow_activity_entity.WorkflowActivities

func (w *ActivityRepository) GetActivitiesByWorkflowIds(ids []int) (ResultGetActivitiesByWorkflowIds, error) {
	db := repository.GetInstance()
	mapWfIdToActivities := make(ResultGetActivitiesByWorkflowIds)

	if len(ids) == 0 {
		return mapWfIdToActivities, nil
	}

	// Monta a lista de IDs no formato SQL: (1, 2, 3)
	var idStrings []string
	for _, id := range ids {
		idStrings = append(idStrings, fmt.Sprintf("%d", id))
	}
	idList := strings.Join(idStrings, ", ")

	query := fmt.Sprintf(
		`SELECT id, workflow_id, namespace, name, image, resource_k8s_base64, status, proc_id, created_at, started_at, finished_at 
		FROM %s WHERE workflow_id IN (%s)`,
		w.tableNameActivity,
		idList,
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

	for _, rowData := range rows {
		row := rowData.([]interface{})
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
			CreatedAt:         toNullableString(data["created_at"]),
			StartedAt:         toNullableString(data["started_at"]),
			FinishedAt:        toNullableString(data["finished_at"]),
		}

		activity := workflow_activity_entity.DatabaseToWorkflowActivities(
			workflow_activity_entity.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfa},
		)

		mapWfIdToActivities[wfa.WorkflowId] = append(mapWfIdToActivities[wfa.WorkflowId], activity)
	}

	return mapWfIdToActivities, nil
}

func toNullableString(val interface{}) *string {
	if val == nil {
		return nil
	}
	str := val.(string)
	return &str
}
