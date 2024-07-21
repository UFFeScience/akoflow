package get_pending_workflow_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
)

type GetPendingWorkflowService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func New() GetPendingWorkflowService {
	return GetPendingWorkflowService{
		namespace:          config.App().DefaultNamespace,
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
	}
}

func (g *GetPendingWorkflowService) GetPendingWorkflows() ([]workflow_entity.Workflow, error) {
	workflows, err := g.retriveWorkflowsOnDatabase()
	if err != nil {
		return nil, err
	}

	return workflows, nil
}

func (g *GetPendingWorkflowService) retriveWorkflowsOnDatabase() ([]workflow_entity.Workflow, error) {
	workflows, err := g.workflowRepository.GetPendingWorkflows(g.namespace)
	if err != nil {
		return nil, err
	}

	ids := getIds(workflows)
	mapWfActivities, err := g.activityRepository.GetActivitiesByWorkflowIds(ids)

	if err != nil {
		return nil, err
	}

	workflows = hydrateWorkflows(workflows, mapWfActivities)
	return workflows, nil
}

func hydrateWorkflows(workflows []workflow_entity.Workflow, mapWfActivities activity_repository.ResultGetActivitiesByWorkflowIds) []workflow_entity.Workflow {
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

func getIds(workflows []workflow_entity.Workflow) []int {
	var ids []int
	for _, wf := range workflows {
		ids = append(ids, wf.Id)
	}
	return ids
}
