package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
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

func (k *KubernetesRuntimeService) ApplyJob(activityID int) {

	wfa, err := k.activityRepository.Find(activityID)
	wf, _ := k.workflowRepository.Find(wfa.WorkflowId)

	if err != nil {
		config.App().Logger.Infof("WORKER: Activity not found %d", activityID)
		return
	}

	modeService := ModeRunActivityService(wf.GetMode()).
		SetWorkflow(wf).
		SetWorkflowActivity(wfa)

	resourceOk := modeService.HandleResourceToRunJob(activityID)
	if resourceOk {
		modeService.ApplyJob(activityID)
	}

	config.App().Logger.Infof("WORKER: Activity %d started", activityID)
}

func (k *KubernetesRuntimeService) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) {
	NewMonitorVerifyActivityWasFinishedService().VerifyActivities(workflow)
}

func (k *KubernetesRuntimeService) GetLogs(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) {
	NewMonitorGetLogsActivityService().GetLogs(wf, wfa)
}

func (k *KubernetesRuntimeService) HealthCheck(runtime string) bool {
	return NewHealthCheckRuntimeK8sService().HealthCheck(runtime)
}
