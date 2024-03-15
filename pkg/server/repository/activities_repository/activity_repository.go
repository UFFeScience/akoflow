package activities_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

type ActivityRepository struct {
	tableNameActivity             string
	tableNameActivityDependencies string
}

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2

var TableNameActivities = "activities"
var ColumnsActivities = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, namespace TEXT, name TEXT, image TEXT, resource_k8s_base64 TEXT, status INTEGER)"

var TableNameActivitiesDependencies = "activities_dependencies"
var ColumnsActivitiesDependencies = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, activity_id INTEGER, depend_on_activity INTEGER)"

func New() *ActivityRepository {

	database := connector.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, TableNameActivities, ColumnsActivities)
	if err != nil {
		return nil
	}

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, TableNameActivitiesDependencies, ColumnsActivitiesDependencies)

	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &ActivityRepository{
		tableNameActivity:             TableNameActivities,
		tableNameActivityDependencies: TableNameActivitiesDependencies,
	}
}

func (w *ActivityRepository) Create(namespace string, workflowId int, image string, activities []workflow.WorkflowActivities) error {
	err := w.createActivity(namespace, workflowId, image, activities)

	if err != nil {
		println("Error creating activity" + err.Error())
		return err
	}

	err = w.createActivityDependency(workflowId, activities)

	if err != nil {
		println("Error creating activity dependency" + err.Error())
		return err
	}

	return nil

}

func (w *ActivityRepository) createActivityDependency(workflowId int, activitiesYaml []workflow.WorkflowActivities) error {

	database := connector.Database{}
	c := database.Connect()

	activitiesDatabase, err := w.GetByWorkflowId(workflowId)

	if err != nil {
		println("Error getting activities by workflow id" + err.Error())
		return err
	}
	mapActivityNameToId := w.createMapActivityNameToId(activitiesDatabase)

	for _, activity := range activitiesYaml {
		if activity.DependsOn == nil {
			continue
		}

		for _, dependOnActivity := range activity.DependsOn {
			activityDependency := mapActivityNameToId[dependOnActivity]
			if activityDependency == 0 {
				println("Activity dependency not found")
				continue
			}

			idActivity := mapActivityNameToId[activity.Name]

			result, err := c.Exec("INSERT INTO "+w.tableNameActivityDependencies+" (workflow_id, activity_id, depend_on_activity) VALUES (?, ?, ?)", workflowId, idActivity, activityDependency)

			if err != nil {
				println("Error creating activity dependency" + err.Error())
				return err
			}

			activityId, _ := result.LastInsertId()

			println("Activity dependency created with id: ", activityId)

		}
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}

func (w *ActivityRepository) createMapActivityNameToId(activities []workflow.WorkflowActivities) map[string]int {
	var mapActivityNameToId = make(map[string]int)
	for _, activity := range activities {
		mapActivityNameToId[activity.Name] = activity.Id
	}
	return mapActivityNameToId
}

func (w *ActivityRepository) findActivityIdByName(name string, activities []workflow.WorkflowActivities) int {
	for _, activity := range activities {
		if activity.Name == name {
			return activity.Id
		}
	}
	return -1
}

func (w *ActivityRepository) createActivity(namespace string, workflowId int, image string, activities []workflow.WorkflowActivities) error {
	database := connector.Database{}
	c := database.Connect()

	for _, activity := range activities {
		rawActivity := activity.GetBase64Activities()

		result, err := c.Exec(
			"INSERT INTO "+w.tableNameActivity+" (workflow_id, namespace, name, image, resource_k8s_base64, status) VALUES (?, ?, ?, ?, ?, ?)",
			workflowId, namespace, activity.Name, image, rawActivity, StatusCreated)

		if err != nil {
			return err
		}

		activityId, _ := result.LastInsertId()

		println("Activity created with id: ", activityId)

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
		rows, err := c.Query("SELECT * FROM "+w.tableNameActivity+" WHERE workflow_id = ?", id)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var wfaDatabase workflow.WorkflowActivityDatabase
			err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
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

func (w *ActivityRepository) UpdateStatus(id int, status int) error {
	database := connector.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE "+w.tableNameActivity+" SET status = ? WHERE ID = ?", status, id)
	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}

func (w *ActivityRepository) Find(id int) (workflow.WorkflowActivities, error) {
	database := connector.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableNameActivity+" WHERE ID = ?", id)
	if err != nil {
		return workflow.WorkflowActivities{}, err
	}

	var wfaDatabase workflow.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
		if err != nil {
			return workflow.WorkflowActivities{}, err
		}
	}

	activity := workflow.DatabaseToWorkflowActivities(workflow.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})

	err = c.Close()
	if err != nil {
		return workflow.WorkflowActivities{}, err
	}

	return activity, nil
}

func (w *ActivityRepository) GetByWorkflowId(id int) ([]workflow.WorkflowActivities, error) {
	database := connector.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+w.tableNameActivity+" WHERE workflow_id = ?", id)
	if err != nil {
		return []workflow.WorkflowActivities{}, err
	}

	var activities []workflow.WorkflowActivities
	var wfaDatabase workflow.WorkflowActivityDatabase
	for rows.Next() {
		err = rows.Scan(&wfaDatabase.Id, &wfaDatabase.WorkflowId, &wfaDatabase.Namespace, &wfaDatabase.Name, &wfaDatabase.Image, &wfaDatabase.ResourceK8sBase64, &wfaDatabase.Status)
		if err != nil {
			return []workflow.WorkflowActivities{}, err
		}

		activity := workflow.DatabaseToWorkflowActivities(workflow.ParamsDatabaseToWorkflowActivities{WorkflowActivityDatabase: wfaDatabase})
		activities = append(activities, activity)

	}

	err = c.Close()
	if err != nil {
		return []workflow.WorkflowActivities{}, err
	}

	return activities, nil

}
