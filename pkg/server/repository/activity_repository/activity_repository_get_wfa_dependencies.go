package activity_repository

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/repository"
)

func (w *ActivityRepository) GetWfaDependencies(workflowId int) ([]workflow_activity_entity.WorkflowActivityDependencyDatabase, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableNameActivityDependencies+" WHERE workflow_id = ?", workflowId)
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

	return wfaDependenciesDatabase, nil

}