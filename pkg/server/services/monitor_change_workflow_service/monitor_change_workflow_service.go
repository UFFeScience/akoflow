package monitor_change_workflow_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_workflow_by_status_service"
)

type MonitorChangeWorkflowService struct {
	namespace                 string
	workflowRepository        *workflow_repository.WorkflowRepository
	activityRepository        *activities_repository.ActivityRepository
	channelManager            *channel.Manager
	getPendingWorkflowService *get_pending_workflow_service.GetPendingWorkflowService
	getWorkflowByStatus       *get_workflow_by_status_service.GetWorkflowByStatusService
}

func New() *MonitorChangeWorkflowService {
	return &MonitorChangeWorkflowService{
		namespace:                 "k8science-cluster-manager",
		workflowRepository:        workflow_repository.New(),
		activityRepository:        activities_repository.New(),
		channelManager:            channel.GetInstance(),
		getPendingWorkflowService: get_pending_workflow_service.New(),
		getWorkflowByStatus:       get_workflow_by_status_service.New(),
	}
}

func (m *MonitorChangeWorkflowService) MonitorChangeWorkflow() {
	wfsPending, _ := m.getPendingWorkflowService.GetPendingWorkflows()

	m.handleVerifyWorkflowWasFinished(wfsPending)

}

func (m *MonitorChangeWorkflowService) handleVerifyWorkflowWasFinished(wfs []workflow.Workflow) {
	for _, wf := range wfs {
		wfaRunning := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activities_repository.StatusRunning)
		wfaCreated := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activities_repository.StatusCreated)
		wfaFinished := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activities_repository.StatusFinished)

		if len(wfaRunning) == 0 && len(wfaCreated) == 0 && len(wfaFinished) == 0 {
			println("Workflow finished: ", wf.Id)
			var _ = m.workflowRepository.UpdateStatus(wf.Id, workflow_repository.StatusFinished)
		}

		if len(wfaRunning) == 0 && len(wfaCreated) == 0 && len(wfaFinished) > 0 {
			println("Workflow finished: ", wf.Id)
			var _ = m.workflowRepository.UpdateStatus(wf.Id, workflow_repository.StatusFinished)
		}

	}
}
