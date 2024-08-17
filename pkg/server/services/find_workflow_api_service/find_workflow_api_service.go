package find_workflow_api_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
)

type FindWorkflowApiService struct {
	workflowRepository workflow_repository.IWorkflowRepository
}

func New() *FindWorkflowApiService {
	return &FindWorkflowApiService{
		workflowRepository: config.App().Repository.WorkflowRepository,
	}
}

func (h *FindWorkflowApiService) FindWorkflowById(id int) (workflow_entity.Workflow, error) {
	wf, err := h.workflowRepository.Find(id)

	if err != nil {
		return workflow_entity.Workflow{}, err
	}

	return wf, nil
}
