package get_pending_workflow_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
)

type GetPendingWorkflowService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activities_repository.IActivityRepository
}

func New() *GetPendingWorkflowService {
	return &GetPendingWorkflowService{
		namespace:          "k8science-cluster-manager",
		workflowRepository: workflow_repository.New(),
		activityRepository: activities_repository.New(),
	}
}

func (g *GetPendingWorkflowService) GetPendingWorkflows() ([]workflow.Workflow, error) {
	workflows, err := g.retriveWorkflowsOnDatabase()
	if err != nil {
		return nil, err
	}

	return workflows, nil
}

func (g *GetPendingWorkflowService) retriveWorkflowsOnDatabase() ([]workflow.Workflow, error) {
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

func hydrateWorkflows(workflows []workflow.Workflow, mapWfActivities activities_repository.ResultGetActivitiesByWorkflowIds) []workflow.Workflow {
	var workflowsToReturn []workflow.Workflow
	for _, wf := range workflows {
		if mapWfActivities[wf.Id] == nil {
			continue
		}
		wf.Spec.Activities = mapWfActivities[wf.Id]
		workflowsToReturn = append(workflowsToReturn, wf)
	}
	return workflowsToReturn
}

func getIds(workflows []workflow.Workflow) []int {
	var ids []int
	for _, wf := range workflows {
		ids = append(ids, wf.Id)
	}
	return ids
}
