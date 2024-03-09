package create_workflow_in_database_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/workflow"
)

type CreateWorkflowInDatabaseService struct {
	namespace          string
	workflowRepository *workflow_repository.WorkflowRepository
	activityRepository *activities_repository.ActivityRepository
}

func New() *CreateWorkflowInDatabaseService {
	return &CreateWorkflowInDatabaseService{
		namespace:          "k8science-cluster-manager",
		workflowRepository: workflow_repository.New(),
		activityRepository: activities_repository.New(),
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

	return nil

}
