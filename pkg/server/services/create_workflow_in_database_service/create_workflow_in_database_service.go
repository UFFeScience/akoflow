package create_workflow_in_database_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/storages_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
)

type CreateWorkflowInDatabaseService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
	storageRepository  storages_repository.IStorageRepository
}

func New() *CreateWorkflowInDatabaseService {
	return &CreateWorkflowInDatabaseService{
		namespace:          "k8science-cluster-manager",
		workflowRepository: workflow_repository.New(),
		activityRepository: activity_repository.New(),
		storageRepository:  storages_repository.New(),
	}
}

func (c *CreateWorkflowInDatabaseService) Create(workflow workflow.Workflow) error {
	workflowId, err := c.workflowRepository.Create(c.namespace, workflow)
	if err != nil {
		return err
	}

	err = c.activityRepository.Create(c.namespace, workflowId, workflow.Spec.Image, workflow.Spec.Activities)
	if err != nil {
		return err
	}

	err = c.storageRepository.Create(storages_repository.ParamsStorageCreate{
		WorkflowId:       workflowId,
		Namespace:        c.namespace,
		Status:           storages_repository.StatusCreated,
		StorageMountPath: workflow.Spec.MountPath,
		StorageClass:     workflow.Spec.StorageClassName,
		StorageSize:      workflow.Spec.StorageSize,
	})

	if err != nil {
		return err
	}

	return nil

}
