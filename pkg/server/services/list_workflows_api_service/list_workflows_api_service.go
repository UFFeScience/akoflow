package list_workflows_api_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/mapper/mapper_engine_api"
	"github.com/ovvesley/akoflow/pkg/server/types/types_api"
	"github.com/ovvesley/akoflow/pkg/server/utils"
)

type ListWorkflowsApiService struct {
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func New() *ListWorkflowsApiService {
	return &ListWorkflowsApiService{
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
	}
}

func (h *ListWorkflowsApiService) ListAllWorkflows() ([]types_api.ApiWorkflowType, error) {
	workflowsEngine, err := h.workflowRepository.ListAllWorkflows(nil)

	if err != nil {
		return nil, err
	}

	ids := utils.GetIds(workflowsEngine)
	mapWfActivities, err := h.activityRepository.GetActivitiesByWorkflowIds(ids)

	if err != nil {
		return nil, err
	}

	workflowsEngine = utils.HydrateWorkflows(workflowsEngine, mapWfActivities)

	workflowApi := mapper_engine_api.MapEngineWorkflowEntityToApiWorkflowEntityList(workflowsEngine)

	return workflowApi, nil

}
