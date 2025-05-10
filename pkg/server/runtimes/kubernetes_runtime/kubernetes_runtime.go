package kubernetes_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/kubernetes_runtime/kubernetes_runtime_service"
)

type KubernetesRuntime struct {
	namespace string

	kubernetesRuntimeService *kubernetes_runtime_service.KubernetesRuntimeService

	runtimeName string
}

func New() *KubernetesRuntime {
	return &KubernetesRuntime{
		namespace:                config.App().DefaultNamespace,
		kubernetesRuntimeService: kubernetes_runtime_service.New(),
	}
}

func (k *KubernetesRuntime) SetRuntimeName(name string) *KubernetesRuntime {
	k.runtimeName = name
	return k
}

func (k *KubernetesRuntime) GetRuntimeName() string {
	return k.runtimeName
}

func (k *KubernetesRuntime) StartConnection() error {
	return nil
}

func (k *KubernetesRuntime) StopConnection() error {
	return nil
}

func (k *KubernetesRuntime) ApplyJob(workflowID int, activityID int) bool {
	k.kubernetesRuntimeService.ApplyJob(activityID)
	return true
}

func (k *KubernetesRuntime) DeleteJob(workflowID int, activityID int) bool {
	return true
}

func (k *KubernetesRuntime) GetMetrics(workflowID int, activityID int) string {
	return ""
}

func (k *KubernetesRuntime) GetLogs(workflow workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities) string {
	k.kubernetesRuntimeService.GetLogs(workflow, workflowActivity)
	return ""
}

func (k *KubernetesRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func (k *KubernetesRuntime) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool {
	k.kubernetesRuntimeService.VerifyActivitiesWasFinished(workflow)
	return true
}

func (k *KubernetesRuntime) HealthCheck() bool {
	return k.kubernetesRuntimeService.HealthCheck(k.runtimeName)
}

func NewKubernetesRuntime() *KubernetesRuntime {
	return &KubernetesRuntime{
		namespace:                config.App().DefaultNamespace,
		kubernetesRuntimeService: kubernetes_runtime_service.New(),
	}
}
