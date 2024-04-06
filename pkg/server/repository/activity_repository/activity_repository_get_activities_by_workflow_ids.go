package activity_repository

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/repository"
)

type ResultGetActivitiesByWorkflowIds map[int][]workflow_activity_entity.WorkflowActivities

func (w *ActivityRepository) GetActivitiesByWorkflowIds(ids []int) (ResultGetActivitiesByWorkflowIds, error) {
	database := repository.Database{}
	c := database.Connect()

	var mapWfIdToActivities = make(ResultGetActivitiesByWorkflowIds)

	for _, id := range ids {
		rows, err := c.Query("SELECT * FROM "+w.tableNameActivity+" WHERE workflow_id = ?", id)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var wfaDatabase workflow_activity_entity.WorkflowActivityDatabase
			err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
			if err != nil {
				return nil, err
			}

			activity := workflow_activity_entity.DatabaseToWorkflowActivities(workflow_activity_entity.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})

			if mapWfIdToActivities[id] == nil {
				mapWfIdToActivities[id] = make([]workflow_activity_entity.WorkflowActivities, 0)
			}

			mapWfIdToActivities[id] = append(mapWfIdToActivities[id], activity)
		}
	}

	err := c.Close()
	if err != nil {
		return nil, err
	}

	return mapWfIdToActivities, nil
}
