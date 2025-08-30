package kubernetes_runtime_service

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/node_metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/node_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type MonitorGetMetricsActivityService struct {
	namespace             string
	logsRepository        logs_repository.ILogsRepository
	metricsRepository     metrics_repository.IMetricsRepository
	nodeMetricsRepository node_metrics_repository.INodeMetricsRepository
	nodeRepository        node_repository.INodeRepository
	workflowRepository    workflow_repository.IWorkflowRepository
	activityRepository    activity_repository.IActivityRepository
	runtimeRepository     runtime_repository.IRuntimeRepository

	connector connector_k8s.IConnector
}

func NewMonitorGetMetricsActivityService() *MonitorGetMetricsActivityService {
	return &MonitorGetMetricsActivityService{
		namespace:             "akoflow",
		logsRepository:        config.App().Repository.LogsRepository,
		metricsRepository:     config.App().Repository.MetricsRepository,
		nodeMetricsRepository: config.App().Repository.NodeMetricsRepository,
		workflowRepository:    config.App().Repository.WorkflowRepository,
		activityRepository:    config.App().Repository.ActivityRepository,
		nodeRepository:        config.App().Repository.NodeRepository,

		runtimeRepository: config.App().Repository.RuntimeRepository,

		connector: config.App().Connector.K8sConnector,
	}
}

func (m *MonitorGetMetricsActivityService) GetMetrics(wf int, wfa int) {
	m.handleGetMetricsByActivity(wf, wfa)
}

func (m *MonitorGetMetricsActivityService) handleGetMetricsByActivity(wfID int, wfaID int) {

	wfa, err := m.activityRepository.Find(wfaID)
	wf, _ := m.workflowRepository.Find(wfID)
	if err != nil {
		config.App().Logger.Infof("WORKER: Activity not found %d", wfa.Id)
		return
	}

	fmt.Println("Activity: ", wfa.WorkflowId, wfa.Id)

	nameJob := wfa.GetNameJob()

	runtime, err := m.runtimeRepository.GetByName(wfa.GetRuntimeId())
	if err != nil {
		return
	}

	job, err := m.connector.Pod(runtime).GetPodByJob(m.namespace, nameJob)
	if err != nil {
		return
	}

	podName, err := job.GetPodName()
	if err != nil {
		return
	}

	m.retrieveMetricsInDatabase(wf, wfa, podName)
}

func (m *MonitorGetMetricsActivityService) retrieveMetricsInDatabase(_ workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities, podName string) {

	runtime, err := m.runtimeRepository.GetByName(wfa.GetRuntimeId())
	if err != nil {
		return
	}

	metric, err := m.connector.Metrics(runtime).GetPodMetrics(m.namespace, podName)
	if err != nil {
		return
	}

	_ = m.metricsRepository.Create(metrics_repository.ParamsMetricsCreate{
		MetricsDatabase: metrics_repository.MetricsDatabase{
			ActivityId: wfa.Id,
			Cpu:        metric.Containers[0].Usage.Cpu,
			Memory:     metric.Containers[0].Usage.Memory,
			Window:     metric.Window,
			Timestamp:  metric.Timestamp.String(),
		},
	})

	config.App().Logger.Infof("WORKER: Metrics collected for activity %d - CPU: %s - Memory: %s", wfa.Id, metric.Containers[0].Usage.Cpu, metric.Containers[0].Usage.Memory)

	nodeMetrics, err := m.connector.Metrics(runtime).GetNodeMetrics()
	if err != nil {
		return
	}

	for _, node := range nodeMetrics {

		nodeDb, err := m.nodeRepository.GetByName(node.Name)
		if err != nil {
			continue
		}
		fmt.Println("Node: ", node.Name, node.CPU, node.Memory)

		_ = m.nodeMetricsRepository.Create(node_metrics_repository.ParamsNodeMetricsCreate{
			NodeMetricsDatabase: node_metrics_repository.NodeMetricsDatabase{
				NodeID:       nodeDb.Name,
				CpuUsage:     node.CPU,
				MemoryUsage:  node.Memory,
				CpuMemory:    "0",
				MemoryLimit:  "0",
				NetworkUsage: "0",
				Timestamp:    metric.Timestamp.String(),
			},
		})
	}

}
