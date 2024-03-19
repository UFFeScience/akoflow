package activity_repository

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/repository"
)

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

func (w *ActivityRepository) createActivity(namespace string, workflowId int, image string, activities []workflow.WorkflowActivities) error {
	database := repository.Database{}
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

func (w *ActivityRepository) createActivityDependency(workflowId int, activitiesYaml []workflow.WorkflowActivities) error {

	database := repository.Database{}
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
