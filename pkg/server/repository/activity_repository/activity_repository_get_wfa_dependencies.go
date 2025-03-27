package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) GetWfaDependencies(workflowId int) ([]workflow_activity_entity.WorkflowActivityDependencyDatabase, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT id, workflow_id, activity_id, depend_on_activity FROM "+w.tableNameActivityDependencies+" WHERE workflow_id = ?", workflowId)
	if err != nil {
		return nil, err
	}

	// array of workflow_entity.WorkflowActivityDependencyDatabase
	var wfaDependenciesDatabase []workflow_activity_entity.WorkflowActivityDependencyDatabase
	for rows.Next() {
		var wfaDependencyDatabase workflow_activity_entity.WorkflowActivityDependencyDatabase
		err = rows.Scan(&wfaDependencyDatabase.Id, &wfaDependencyDatabase.WorkflowId, &wfaDependencyDatabase.ActivityId, &wfaDependencyDatabase.DependsOnId)
		if err != nil {
			return nil, err
		}
		wfaDependenciesDatabase = append(wfaDependenciesDatabase, wfaDependencyDatabase)
	}

	err = c.Close()
	if err != nil {
		return nil, err
	}

	return wfaDependenciesDatabase, nil

}
