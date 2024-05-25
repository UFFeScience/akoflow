package create_workflow_in_database_service

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/create_storage_in_database_service"
)

type CreateWorkflowInDatabaseService struct {
	namespace                      string
	workflowRepository             workflow_repository.IWorkflowRepository
	activityRepository             activity_repository.IActivityRepository
	storageRepository              storages_repository.IStorageRepository
	createStorageInDatabaseService create_storage_in_database_service.CreateStorageInDatabaseService
}

func New() *CreateWorkflowInDatabaseService {
	return &CreateWorkflowInDatabaseService{
		namespace:                      "akoflow",
		workflowRepository:             workflow_repository.New(),
		activityRepository:             activity_repository.New(),
		storageRepository:              storages_repository.New(),
		createStorageInDatabaseService: create_storage_in_database_service.New(),
	}
}

func (c *CreateWorkflowInDatabaseService) Create(workflow workflow_entity.Workflow) error {
	workflowId, err := c.workflowRepository.Create(c.namespace, workflow)
	if err != nil {
		return err
	}

	err = c.activityRepository.Create(c.namespace, workflowId, workflow.Spec.Image, workflow.Spec.Activities)
	if err != nil {
		return err
	}

	err = c.createStorageInDatabaseService.CreateByWorkflow(workflowId)
	if err != nil {
		return err
	}

	return nil

}
