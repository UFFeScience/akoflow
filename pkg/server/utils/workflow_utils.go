package utils

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
)

func GetIds(workflows []workflow_entity.Workflow) []int {
	var ids []int
	for _, wf := range workflows {
		ids = append(ids, wf.Id)
	}
	return ids
}

func HydrateWorkflows(workflows []workflow_entity.Workflow, mapWfActivities activity_repository.ResultGetActivitiesByWorkflowIds) []workflow_entity.Workflow {
	var workflowsToReturn []workflow_entity.Workflow
	for _, wf := range workflows {
		if mapWfActivities[wf.Id] == nil {
			continue
		}
		wf.Spec.Activities = mapWfActivities[wf.Id]
		workflowsToReturn = append(workflowsToReturn, wf)
	}
	return workflowsToReturn
}
