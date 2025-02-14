package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type ResultGetActivitiesByWorkflowIds map[int][]workflow_activity_entity.WorkflowActivities

func (w *ActivityRepository) GetActivitiesByWorkflowIds(ids []int) (ResultGetActivitiesByWorkflowIds, error) {

	var mapWfIdToActivities = make(ResultGetActivitiesByWorkflowIds)

	for _, id := range ids {
		database := repository.Database{}
		c := database.Connect()
		rows, err := c.Query("SELECT id, workflow_id, namespace, name, image, resource_k8s_base64, status, proc_id, created_at, started_at, finished_at FROM activities WHERE workflow_id = ?", id)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var wfaDatabase workflow_activity_entity.WorkflowActivityDatabase
			err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status, &wfaDatabase.ProcId, &wfaDatabase.CreatedAt, &wfaDatabase.StartedAt, &wfaDatabase.FinishedAt)
			if err != nil {
				return nil, err
			}

			activity := workflow_activity_entity.DatabaseToWorkflowActivities(workflow_activity_entity.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})

			if mapWfIdToActivities[id] == nil {
				mapWfIdToActivities[id] = make([]workflow_activity_entity.WorkflowActivities, 0)
			}

			mapWfIdToActivities[id] = append(mapWfIdToActivities[id], activity)
		}
		err = c.Close()
		if err != nil {
			return nil, err
		}
	}

	return mapWfIdToActivities, nil
}
