package monitor_change_workflow_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_workflow_by_status_service"
)

type MonitorChangeWorkflowService struct {
	namespace                 string
	workflowRepository        workflow_repository.IWorkflowRepository
	activityRepository        activity_repository.IActivityRepository
	channelManager            *channel.Manager
	getPendingWorkflowService *get_pending_workflow_service.GetPendingWorkflowService
	getWorkflowByStatus       *get_workflow_by_status_service.GetWorkflowByStatusService
	connector                 connector.IConnector
}

func New() *MonitorChangeWorkflowService {
	return &MonitorChangeWorkflowService{
		namespace:                 "k8science-cluster-manager",
		workflowRepository:        workflow_repository.New(),
		activityRepository:        activity_repository.New(),
		channelManager:            channel.GetInstance(),
		getPendingWorkflowService: get_pending_workflow_service.New(),
		getWorkflowByStatus:       get_workflow_by_status_service.New(),
		connector:                 connector.New(),
	}
}

func (m *MonitorChangeWorkflowService) MonitorChangeWorkflow() {
	wfsPending, _ := m.getPendingWorkflowService.GetPendingWorkflows()

	m.handleVerifyWorkflowWasFinished(wfsPending)
	m.handleVerifyWorkflowActivitiesWasFinished(wfsPending)

}

func (m *MonitorChangeWorkflowService) handleVerifyWorkflowWasFinished(wfs []workflow.Workflow) {
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

func (m *MonitorChangeWorkflowService) handleVerifyWorkflowActivitiesWasFinished(wfs []workflow.Workflow) {
	for _, wf := range wfs {
		for _, activity := range wf.Spec.Activities {
			m.handleVerifyActivityWasFinished(activity, wf)
		}
	}
}

// [TODO] Verificação de Status das atividades muito simplista. Deve ser melhorada.
func (m *MonitorChangeWorkflowService) handleVerifyActivityWasFinished(activity workflow.WorkflowActivities, wf workflow.Workflow) int {
	println("Verifying activity: ", activity.Name, " with id: ", activity.Id)

	wfaDatabase, _ := m.activityRepository.Find(activity.Id)

	println("Activity status Database: ", wfaDatabase.Status)

	jobResponse, _ := m.connector.Job().GetJob(m.namespace, activity.GetName())

	if jobResponse.Status.Active == 1 {
		return activity_repository.StatusRunning
	}

	if jobResponse.Status.Succeeded == 1 {
		var _ = m.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusFinished)
		return activity_repository.StatusFinished
	}

	if jobResponse.Metadata.Name == "" {
		println("Activity not send to k8s yet. Go back to created status")
		var _ = m.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusCreated)
		return activity_repository.StatusCreated
	}

	return activity_repository.StatusFinished

}
