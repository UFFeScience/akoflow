package kubernetes_runtime_service

import (
	"errors"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_job_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type MonitorVerifyActivityWasFinishedService struct {
	namespace string

	activityRepository activity_repository.IActivityRepository
	runtimeRepository  runtime_repository.IRuntimeRepository
	logsRepository     logs_repository.ILogsRepository

	connector connector_k8s.IConnector
}

func NewMonitorVerifyActivityWasFinishedService() *MonitorVerifyActivityWasFinishedService {
	return &MonitorVerifyActivityWasFinishedService{
		namespace:          "akoflow",
		activityRepository: config.App().Repository.ActivityRepository,
		logsRepository:     config.App().Repository.LogsRepository,
		connector:          config.App().Connector.K8sConnector,
	}
}

func (m *MonitorVerifyActivityWasFinishedService) VerifyActivities(wf workflow_entity.Workflow) {
	for _, activity := range wf.Spec.Activities {
		if activity.HasDependencies() && activity.Status == activity_repository.StatusCreated {
			m.handleVerifyPreActivityWasFinished(activity, wf)
		}
	}

	for _, activity := range wf.Spec.Activities {
		m.handleVerifyActivityWasFinished(activity, wf)
	}
}

func (m *MonitorVerifyActivityWasFinishedService) handleVerifyPreActivityWasFinished(activity workflow_activity_entity.WorkflowActivities, wf workflow_entity.Workflow) int {
	preactivity, _ := m.activityRepository.FindPreActivity(activity.Id)

	runtime, err := m.runtimeRepository.GetByName(wf.GetRuntimeId())
	if err != nil {
		return activity_repository.StatusCreated
	}

	jobResponse, err := m.connector.Job(runtime).GetJob(m.namespace, activity.GetPreActivityName())

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

// [TODO] Verificação de Status das atividades muito simplista. Deve ser melhorada.
func (m *MonitorVerifyActivityWasFinishedService) handleVerifyActivityWasFinished(activity workflow_activity_entity.WorkflowActivities, wf workflow_entity.Workflow) int {
	println("Verifying activity: ", activity.Name, " with id: ", activity.Id)

	wfaDatabase, _ := m.activityRepository.Find(activity.Id)

	println("Activity status Database: ", wfaDatabase.Status)

	if wfaDatabase.Status == activity_repository.StatusFinished {
		return activity_repository.StatusFinished
	}

	if wfaDatabase.Status == activity_repository.StatusCreated {
		return activity_repository.StatusCreated
	}

	runtime, err := m.runtimeRepository.GetByName(wf.GetRuntimeId())
	if err != nil {
		return activity_repository.StatusCreated
	}

	jobResponse, _ := m.connector.Job(runtime).GetJob(m.namespace, activity.GetNameJob())

	if jobResponse.Status.Active == 1 {
		return activity_repository.StatusRunning
	}

	if jobResponse.Status.Succeeded == 1 {
		var _ = m.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusFinished)
		m.monitorHandleLogs(wf, activity)
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
		m.monitorHandleLogs(wf, activity)

		return activity_repository.StatusFinished
	}

	return activity_repository.StatusFinished

}

func (m *MonitorVerifyActivityWasFinishedService) monitorHandleLogs(wf workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) {

	runtime, err := m.runtimeRepository.GetByName(wf.GetRuntimeId())
	if err != nil {
		println("Runtime not found")
		return
	}

	podJob, _ := m.connector.Pod(runtime).GetPodByJob(m.namespace, activity.GetNameJob())
	podName, _ := podJob.GetPodName()

	logs, _ := m.connector.Pod(runtime).GetPodLogs(m.namespace, podName)

	_ = m.logsRepository.Create(logs_repository.ParamsLogsCreate{
		LogsDatabase: logs_repository.LogsDatabase{
			ActivityId: activity.Id,
			Logs:       logs,
		},
	})
}
