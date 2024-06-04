package monitor_change_workflow_service

import (
	"errors"
	"github.com/ovvesley/akoflow/pkg/server/repository/logs_repository"

	"github.com/ovvesley/akoflow/pkg/server/channel"
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_job_k8s"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_workflow_by_status_service"
)

type MonitorChangeWorkflowService struct {
	namespace                 string
	workflowRepository        workflow_repository.IWorkflowRepository
	activityRepository        activity_repository.IActivityRepository
	logsRepository            logs_repository.ILogsRepository
	channelManager            *channel.Manager
	getPendingWorkflowService *get_pending_workflow_service.GetPendingWorkflowService
	getWorkflowByStatus       *get_workflow_by_status_service.GetWorkflowByStatusService
	connector                 connector.IConnector
}

func New() *MonitorChangeWorkflowService {
	return &MonitorChangeWorkflowService{
		namespace:                 "akoflow",
		workflowRepository:        workflow_repository.New(),
		activityRepository:        activity_repository.New(),
		channelManager:            channel.GetInstance(),
		getPendingWorkflowService: get_pending_workflow_service.New(),
		logsRepository:            logs_repository.New(),
		getWorkflowByStatus:       get_workflow_by_status_service.New(),
		connector:                 connector.New(),
	}
}

func (m *MonitorChangeWorkflowService) MonitorChangeWorkflow() {
	wfsPending, _ := m.getPendingWorkflowService.GetPendingWorkflows()

	m.handleVerifyWorkflowWasFinished(wfsPending)
	m.handleVerifyWorkflowPreActivitiesWasFinished(wfsPending)
	m.handleVerifyWorkflowActivitiesWasFinished(wfsPending)

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

func (m *MonitorChangeWorkflowService) handleVerifyWorkflowPreActivitiesWasFinished(wfs []workflow_entity.Workflow) {
	for _, wf := range wfs {
		for _, activity := range wf.Spec.Activities {
			if activity.HasDependencies() && activity.Status == activity_repository.StatusCreated {
				m.handleVerifyPreActivityWasFinished(activity, wf)
			}
		}
	}
}

func (m *MonitorChangeWorkflowService) handleVerifyPreActivityWasFinished(activity workflow_activity_entity.WorkflowActivities, wf workflow_entity.Workflow) int {
	preactivity, _ := m.activityRepository.FindPreActivity(activity.Id)

	jobResponse, err := m.connector.Job().GetJob(m.namespace, activity.GetPreActivityName())

	if errors.Is(err, connector_job_k8s.ErrJobNotFound) {
		println("Job not found")

		preactivity.Status = activity_repository.StatusCreated
		m.activityRepository.UpdatePreActivity(activity.Id, preactivity)
		return activity_repository.StatusCreated

	}
	if err != nil {
		println("Error getting preactivity job, change status to created")
		return activity_repository.StatusCreated
	}

	if jobResponse.Status.Active == 1 {
		return activity_repository.StatusRunning
	}

	if jobResponse.Status.Succeeded == 1 {
		preactivity.Status = activity_repository.StatusFinished
		m.activityRepository.UpdatePreActivity(activity.Id, preactivity)
		return activity_repository.StatusFinished
	}

	if jobResponse.Metadata.Name == "" {
		println("PreActivity not send to k8s yet. Go back to created status")
		preactivity.Status = activity_repository.StatusCreated
		m.activityRepository.UpdatePreActivity(activity.Id, preactivity)
		return activity_repository.StatusCreated
	}

	// temporary solution to handle failed preactivitu=y in k8s. Failed activities will be marked as finished.
	// [TODO] Implement a better solution to handle failed activities.
	if jobResponse.Status.Failed >= 1 {
		println("PreActivity failed: ", activity.Name)
		preactivity.Status = activity_repository.StatusFinished
		errorMessage := "Preactivity failed"
		preactivity.Log = &errorMessage
		m.activityRepository.UpdatePreActivity(activity.Id, preactivity)
		return activity_repository.StatusFinished

	}

	return activity_repository.StatusFinished
}

func (m *MonitorChangeWorkflowService) handleVerifyWorkflowActivitiesWasFinished(wfs []workflow_entity.Workflow) {
	for _, wf := range wfs {
		for _, activity := range wf.Spec.Activities {
			m.handleVerifyActivityWasFinished(activity, wf)
		}
	}
}

// [TODO] Verificação de Status das atividades muito simplista. Deve ser melhorada.
func (m *MonitorChangeWorkflowService) handleVerifyActivityWasFinished(activity workflow_activity_entity.WorkflowActivities, wf workflow_entity.Workflow) int {
	println("Verifying activity: ", activity.Name, " with id: ", activity.Id)

	wfaDatabase, _ := m.activityRepository.Find(activity.Id)

	println("Activity status Database: ", wfaDatabase.Status)

	if wfaDatabase.Status == activity_repository.StatusFinished {
		return activity_repository.StatusFinished
	}

	if wfaDatabase.Status == activity_repository.StatusCreated {
		return activity_repository.StatusCreated
	}

	jobResponse, _ := m.connector.Job().GetJob(m.namespace, activity.GetNameJob())

	if jobResponse.Status.Active == 1 {
		return activity_repository.StatusRunning
	}

	if jobResponse.Status.Succeeded == 1 {
		var _ = m.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusFinished)
		m.monitorHandleLogs(activity)
		return activity_repository.StatusFinished
	}

	if jobResponse.Metadata.Name == "" {
		println("Activity not send to k8s yet. Go back to created status")
		var _ = m.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusCreated)
		return activity_repository.StatusCreated
	}

	// temporary solution to handle failed activities in k8s. Failed activities will be marked as finished.
	// [TODO] Implement a better solution to handle failed activities.
	if jobResponse.Status.Failed >= 1 {
		println("Activity failed: ", activity.Name)
		var _ = m.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusFinished)
		m.monitorHandleLogs(activity)

		return activity_repository.StatusFinished
	}

	return activity_repository.StatusFinished

}

func (m *MonitorChangeWorkflowService) monitorHandleLogs(activity workflow_activity_entity.WorkflowActivities) {
	podJob, _ := m.connector.Pod().GetPodByJob(m.namespace, activity.GetNameJob())
	podName, _ := podJob.GetPodName()

	logs, _ := m.connector.Pod().GetPodLogs(m.namespace, podName)

	var _ = m.logsRepository.Create(logs_repository.ParamsLogsCreate{
		LogsDatabase: logs_repository.LogsDatabase{
			ActivityId: activity.Id,
			Logs:       logs,
		},
	})
}
