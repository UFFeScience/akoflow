package kubernetes_runtime

type KubernetesRuntime struct {
}

func New() *KubernetesRuntime {
	return &KubernetesRuntime{}
}

func (k *KubernetesRuntime) StartConnection() error {
	return nil
}

func (k *KubernetesRuntime) StopConnection() error {
	return nil
}

func (k *KubernetesRuntime) ApplyJob(workflowID int, activityID int) bool {
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
