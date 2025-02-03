package kubernetes_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/kubernetes_runtime/kubernetes_runtime_service"
)

type KubernetesRuntime struct {
	namespace                string
	kubernetesRuntimeService *kubernetes_runtime_service.KubernetesRuntimeService
}

func New() *KubernetesRuntime {
	return &KubernetesRuntime{
		namespace:                config.App().DefaultNamespace,
		kubernetesRuntimeService: kubernetes_runtime_service.New(),
	}
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

func (k *KubernetesRuntime) GetLogs(workflowID int, activityID int) string {
	return ""
}

func (k *KubernetesRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func NewKubernetesRuntime() *KubernetesRuntime {
	return &KubernetesRuntime{}
}
