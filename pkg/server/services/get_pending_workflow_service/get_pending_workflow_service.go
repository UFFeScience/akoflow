package get_pending_workflow_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/utils/utils_workflow"
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

	ids := utils_workflow.GetIds(workflows)
	mapWfActivities, err := g.activityRepository.GetActivitiesByWorkflowIds(ids)

	if err != nil {
		return nil, err
	}

	workflows = utils_workflow.HydrateWorkflows(workflows, mapWfActivities)
	return workflows, nil
}
