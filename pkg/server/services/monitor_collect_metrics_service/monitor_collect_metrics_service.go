package monitor_collect_metrics_service

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/runtimes"
	"github.com/ovvesley/akoflow/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_workflow_by_status_service"
)

type MonitorCollectMetricsService struct {
	getPendingWorkflowService get_pending_workflow_service.GetPendingWorkflowService
	getWorkflowByStatus       get_workflow_by_status_service.GetWorkflowByStatusService
}

func New() *MonitorCollectMetricsService {
	return &MonitorCollectMetricsService{
		getPendingWorkflowService: get_pending_workflow_service.New(),
		getWorkflowByStatus:       get_workflow_by_status_service.New(),
	}
}

func (m *MonitorCollectMetricsService) CollectMetrics() {
	wfsPending, _ := m.getPendingWorkflowService.GetPendingWorkflows()

	for _, wf := range wfsPending {
		m.handleCollectMetricsByWorkflow(wf)
	}
}

func (m *MonitorCollectMetricsService) handleCollectMetricsByWorkflow(wf workflow_entity.Workflow) {
	wfaRunning := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusRunning)

	println("Workflow: ", wf.Id)
	println("Running: ", len(wfaRunning))

	for _, a := range wfaRunning {
		runtimeService := runtimes.GetRuntimeInstance(a.GetRuntimeId())
		runtimeService.GetLogs(wf, a)
		runtimeService.GetMetrics(wf.Id, a.Id)
	}
}
