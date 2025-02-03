package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
)

type KubernetesRuntimeService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func New() *KubernetesRuntimeService {
	return &KubernetesRuntimeService{
		namespace:          config.App().DefaultNamespace,
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
	}
}
