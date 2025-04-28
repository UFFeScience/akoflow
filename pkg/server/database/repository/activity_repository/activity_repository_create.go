package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
)

func (w *ActivityRepository) Create(namespace string, workflowId int, image string, activities []workflow_activity_entity.WorkflowActivities) error {
	err := w.createActivity(namespace, workflowId, image, activities)

	if err != nil {
		println("Error creating activity" + err.Error())
		return err
	}

	err = w.createPreactivity(namespace, workflowId, image, activities)

	err = w.createActivityDependency(workflowId, activities)

	if err != nil {
		println("Error creating activity dependency" + err.Error())
		return err
	}

	return nil

}

// createPreactivity creates preactivity
// if activity has depends_on, it will create preactivity
// this preactivity will be executed before the activity
// is responsible for garanted that all data needed by activity is available before running it
func (w *ActivityRepository) createPreactivity(namespace string, workflowId int, image string, activities []workflow_activity_entity.WorkflowActivities) error {
	database := repository.Database{}
	c := database.Connect()

	activitiesDatabase, err := w.GetByWorkflowId(workflowId)

	if err != nil {
		println("Error getting activities by workflow_entity id" + err.Error())
		return err
	}
	mapActivityNameToId := w.createMapActivityNameToId(activitiesDatabase)

	for _, activity := range activities {
		if activity.DependsOn == nil {
			continue
		}
		activityId := mapActivityNameToId[activity.Name]

		result, err := c.Exec(
			"INSERT INTO "+w.tableNamePreActivity+" (activity_id, workflow_id, namespace, name, resource_k8s_base64, status, log) VALUES (?, ?, ?, ?, ?, ?, ?)",
			activityId, workflowId, namespace, "preactivity-"+activity.Name, nil, StatusCreated, nil)

		if err != nil {
			return err
		}

		preActivityId, _ := result.LastInsertId()

		println("Preactivity created with id: ", preActivityId)

	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}

func (w *ActivityRepository) createActivity(namespace string, workflowId int, image string, activities []workflow_activity_entity.WorkflowActivities) error {

	for _, activity := range activities {

		database := repository.Database{}
		c := database.Connect()

		rawActivity := activity.GetBase64Activities()

		result, err := c.Exec(
			"INSERT INTO "+w.tableNameActivity+" (workflow_id, namespace, name, image, resource_k8s_base64, status, created_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
			workflowId, namespace, activity.Name, image, rawActivity, StatusCreated)

		err = c.Close()

		if err != nil {
			return err
		}

		activityId, _ := result.LastInsertId()

		println("Activity created with id: ", activityId)

	}

	return nil
}

func (w *ActivityRepository) createActivityDependency(workflowId int, activitiesYaml []workflow_activity_entity.WorkflowActivities) error {

	database := repository.Database{}
	c := database.Connect()

	activitiesDatabase, err := w.GetByWorkflowId(workflowId)

	if err != nil {
		println("Error getting activities by workflow_entity id" + err.Error())
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

func (w *ActivityRepository) createMapActivityNameToId(activities []workflow_activity_entity.WorkflowActivities) map[string]int {
	var mapActivityNameToId = make(map[string]int)
	for _, activity := range activities {
		mapActivityNameToId[activity.Name] = activity.Id
	}
	return mapActivityNameToId
}
