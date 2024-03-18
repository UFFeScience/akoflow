package monitor_collect_metrics_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/logs_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/metrics_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_workflow_by_status_service"
)

type MonitorCollectMetricsService struct {
	namespace                 string
	workflowRepository        *workflow_repository.WorkflowRepository
	activityRepository        *activities_repository.ActivityRepository
	metricsRepository         *metrics_repository.MetricsRepository
	logsRepository            *logs_repository.LogsRepository
	getPendingWorkflowService *get_pending_workflow_service.GetPendingWorkflowService
	getWorkflowByStatus       *get_workflow_by_status_service.GetWorkflowByStatusService
	connector                 *connector.Connector
}

func New() *MonitorCollectMetricsService {
	return &MonitorCollectMetricsService{
		namespace:                 "k8science-cluster-manager",
		workflowRepository:        workflow_repository.New(),
		activityRepository:        activities_repository.New(),
		metricsRepository:         metrics_repository.New(),
		logsRepository:            logs_repository.New(),
		getPendingWorkflowService: get_pending_workflow_service.New(),
		getWorkflowByStatus:       get_workflow_by_status_service.New(),
		connector:                 connector.New(),
	}
}

func (m *MonitorCollectMetricsService) CollectMetrics() {
	wfsPending, _ := m.getPendingWorkflowService.GetPendingWorkflows()

	for _, wf := range wfsPending {
		m.handleCollectMetricsByWorkflow(wf)
	}
}

func (m *MonitorCollectMetricsService) handleCollectMetricsByWorkflow(wf workflow.Workflow) {
	wfaRunning := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activities_repository.StatusRunning)

	println("Workflow: ", wf.Id)
	println("Running: ", len(wfaRunning))

	for _, a := range wfaRunning {
		m.handleCollectMetricsByActivity(a)
	}
}

func (m *MonitorCollectMetricsService) handleCollectMetricsByActivity(wfa workflow.WorkflowActivities) {
	println("Activity: ", wfa.WorkflowId, wfa.Id)

	nameJob := wfa.GetName()

	job, err := m.connector.GetPodByJob(m.namespace, nameJob)
	if err != nil {
		return
	}

	podName, err := job.GetPodName()

	if err != nil {
		return
	}

	m.retrieveSaveMetricsInDatabase(wfa, podName)
	m.retrieveSaveLogsInDatabase(wfa, podName)

}

func (m *MonitorCollectMetricsService) retrieveSaveMetricsInDatabase(wfa workflow.WorkflowActivities, podName string) {

	metricsResponse, err := m.connector.GetPodMetrics(m.namespace, podName)
	metricsByPod, err := metricsResponse.GetMetrics()
	metricsByPod.ActivityId = &wfa.Id

	if err != nil {
		println("Error getting metrics")
		return
	}

	_ = m.metricsRepository.Create(metrics_repository.ParamsMetricsCreate{
		MetricsDatabase: metrics_repository.MetricsDatabase{
			ActivityId: wfa.Id,
			Cpu:        metricsByPod.Cpu,
			Memory:     metricsByPod.Memory,
			Window:     metricsByPod.Window,
			Timestamp:  metricsByPod.Timestamp,
		},
	})
}

func (m *MonitorCollectMetricsService) retrieveSaveLogsInDatabase(wfa workflow.WorkflowActivities, podName string) {
	logs, err := m.connector.GetPodLogs(m.namespace, podName)
	if err != nil {
		return
	}

	_ = m.logsRepository.CreateOrUpdate(logs_repository.ParamsLogsCreate{
		LogsDatabase: logs_repository.LogsDatabase{
			ActivityId: wfa.Id,
			Logs:       logs,
		},
	})

}
