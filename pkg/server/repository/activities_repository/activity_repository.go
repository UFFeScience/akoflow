package activities_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

type ActivityRepository struct {
	tableName string
}

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2

var TableName = "activities"
var Columns = "(ID INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, namespace TEXT, name TEXT, image TEXT, resource_k8s_base64 TEXT, status INTEGER, depend_on_activity INTEGER)"

func New() *ActivityRepository {

	database := connector.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, TableName, Columns)
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &ActivityRepository{
		tableName: TableName,
	}
}

func (w *ActivityRepository) Create(namespace string, workflowId int, image string, activities []workflow.WorkflowActivities) error {

	database := connector.Database{}
	c := database.Connect()

	var lastActivity *int64 = nil
	for _, activity := range activities {
		rawActivity := activity.GetBase64Activities()

		result, err := c.Exec(
			"INSERT INTO "+w.tableName+" (workflow_id, namespace, name, image, resource_k8s_base64, status, depend_on_activity) VALUES (?, ?, ?, ?, ?, ?, ?)",
			workflowId, namespace, activity.Name, image, rawActivity, StatusCreated, lastActivity)

		if err != nil {
			return err
		}
		lastInsertId, _ := result.LastInsertId()
		lastActivity = &lastInsertId

	}

	err := c.Close()

	if err != nil {
		return err
	}
	return nil
}

type ResultGetActivitiesByWorkflowIds map[int][]workflow.WorkflowActivities

func (w *ActivityRepository) GetActivitiesByWorkflowIds(ids []int) (ResultGetActivitiesByWorkflowIds, error) {
	database := connector.Database{}
	c := database.Connect()

	var mapWfIdToActivities = make(ResultGetActivitiesByWorkflowIds)

	for _, id := range ids {
		rows, err := c.Query("SELECT * FROM "+w.tableName+" WHERE workflow_id = ?", id)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var wfaDatabase workflow.WorkflowActivityDatabase
			err = rows.Scan(&wfaDatabase.ID, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status, &wfaDatabase.DependOnActivity)
			if err != nil {
				return nil, err
			}

			activity := workflow.DatabaseToWorkflowActivities(workflow.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})

			if mapWfIdToActivities[id] == nil {
				mapWfIdToActivities[id] = make([]workflow.WorkflowActivities, 0)
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
