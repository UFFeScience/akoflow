package activity_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) Create(namespace string, workflowId int, image string, activities []workflow_activity_entity.WorkflowActivities) error {
	if err := w.createActivity(namespace, workflowId, image, activities); err != nil {
		println("Error creating activity: " + err.Error())
		return err
	}

	if err := w.createPreactivity(namespace, workflowId, image, activities); err != nil {
		println("Error creating preactivity: " + err.Error())
		return err
	}

	if err := w.createActivityDependency(workflowId, activities); err != nil {
		println("Error creating activity dependency: " + err.Error())
		return err
	}

	return nil
}

func (w *ActivityRepository) createActivity(namespace string, workflowId int, image string, activities []workflow_activity_entity.WorkflowActivities) error {
	db := repository.GetInstance()

	for _, activity := range activities {
		rawActivity := activity.GetBase64Activities()

		query := fmt.Sprintf(
			"INSERT INTO %s (workflow_id, namespace, name, image, resource_k8s_base64, status, created_at) VALUES (%d, '%s', '%s', '%s', '%s', %d, CURRENT_TIMESTAMP)",
			w.tableNameActivity,
			workflowId,
			namespace,
			activity.Name,
			image,
			rawActivity,
			StatusCreated,
		)

		resp, err := db.Exec(query)
		if err != nil {
			return err
		}

		if len(resp["results"].([]interface{})) > 0 {
			id := int(resp["results"].([]interface{})[0].(map[string]interface{})["last_insert_id"].(float64))
			println("Activity created with id:", id)
		}
	}

	return nil
}

func (w *ActivityRepository) createPreactivity(namespace string, workflowId int, image string, activities []workflow_activity_entity.WorkflowActivities) error {
	db := repository.GetInstance()

	activitiesDatabase, err := w.GetByWorkflowId(workflowId)
	if err != nil {
		println("Error getting activities by workflow id: " + err.Error())
		return err
	}

	mapActivityNameToId := w.createMapActivityNameToId(activitiesDatabase)

	for _, activity := range activities {
		if activity.DependsOn == nil {
			continue
		}
		activityId := mapActivityNameToId[activity.Name]

		query := fmt.Sprintf(
			"INSERT INTO %s (activity_id, workflow_id, namespace, name, resource_k8s_base64, status, log) VALUES (%d, %d, '%s', '%s', NULL, %d, NULL)",
			w.tableNamePreActivity,
			activityId,
			workflowId,
			namespace,
			"preactivity-"+activity.Name,
			StatusCreated,
		)

		resp, err := db.Exec(query)
		if err != nil {
			return err
		}

		if len(resp["results"].([]interface{})) > 0 {
			id := int(resp["results"].([]interface{})[0].(map[string]interface{})["last_insert_id"].(float64))
			println("Preactivity created with id:", id)
		}
	}

	return nil
}

func (w *ActivityRepository) createActivityDependency(workflowId int, activitiesYaml []workflow_activity_entity.WorkflowActivities) error {
	db := repository.GetInstance()

	activitiesDatabase, err := w.GetByWorkflowId(workflowId)
	if err != nil {
		println("Error getting activities by workflow id: " + err.Error())
		return err
	}

	mapActivityNameToId := w.createMapActivityNameToId(activitiesDatabase)

	for _, activity := range activitiesYaml {
		if activity.DependsOn == nil {
			continue
		}

		for _, dependOnActivity := range activity.DependsOn {
			dependencyId := mapActivityNameToId[dependOnActivity]
			if dependencyId == 0 {
				println("Activity dependency not found")
				continue
			}

			activityId := mapActivityNameToId[activity.Name]

			query := fmt.Sprintf(
				"INSERT INTO %s (workflow_id, activity_id, depend_on_activity) VALUES (%d, %d, %d)",
				w.tableNameActivityDependencies,
				workflowId,
				activityId,
				dependencyId,
			)

			resp, err := db.Exec(query)
			if err != nil {
				println("Error creating activity dependency: " + err.Error())
				return err
			}

			if len(resp["results"].([]interface{})) > 0 {
				id := int(resp["results"].([]interface{})[0].(map[string]interface{})["last_insert_id"].(float64))
				println("Activity dependency created with id:", id)
			}
		}
	}

	return nil
}

func (w *ActivityRepository) createMapActivityNameToId(activities []workflow_activity_entity.WorkflowActivities) map[string]int {
	m := make(map[string]int)
	for _, activity := range activities {
		m[activity.Name] = activity.Id
	}
	return m
}
