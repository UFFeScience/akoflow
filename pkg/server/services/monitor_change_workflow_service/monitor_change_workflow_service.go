package monitor_change_workflow_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/runtimes"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_workflow_by_status_service"
)

type MonitorChangeWorkflowService struct {
	namespace                 string
	workflowRepository        workflow_repository.IWorkflowRepository
	getPendingWorkflowService get_pending_workflow_service.GetPendingWorkflowService
	getWorkflowByStatus       get_workflow_by_status_service.GetWorkflowByStatusService
}

func New() *MonitorChangeWorkflowService {
	return &MonitorChangeWorkflowService{
		namespace:          "akoflow",
		workflowRepository: config.App().Repository.WorkflowRepository,

		getPendingWorkflowService: get_pending_workflow_service.New(),
		getWorkflowByStatus:       get_workflow_by_status_service.New(),
	}
}

func (m *MonitorChangeWorkflowService) MonitorChangeWorkflow() {
	wfsPending, _ := m.getPendingWorkflowService.GetPendingWorkflows()
	m.handleVerifyWorkflowWasFinished(wfsPending)
	m.handleVerifyWorkflowActivities(wfsPending)
}

func (m *MonitorChangeWorkflowService) handleVerifyWorkflowActivities(wfs []workflow_entity.Workflow) {

	for _, wf := range wfs {
		runtimes.
			GetRuntimeInstance(wf.GetRuntimeId()).
			VerifyActivitiesWasFinished(wf)
	}
}

func (m *MonitorChangeWorkflowService) handleVerifyWorkflowWasFinished(wfs []workflow_entity.Workflow) {
	for _, wf := range wfs {
		wfaRunning := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusRunning)
		wfaCreated := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusCreated)
		wfaFinished := m.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusFinished)

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
