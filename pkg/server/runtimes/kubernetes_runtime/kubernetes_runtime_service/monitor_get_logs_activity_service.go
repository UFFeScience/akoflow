package kubernetes_runtime_service

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type MonitorGetLogsActivityService struct {
	namespace         string
	logsRepository    logs_repository.ILogsRepository
	metricsRepository metrics_repository.IMetricsRepository

	runtimeRepository runtime_repository.IRuntimeRepository

	connector connector_k8s.IConnector
}

func NewMonitorGetLogsActivityService() *MonitorGetLogsActivityService {
	return &MonitorGetLogsActivityService{
		namespace:         "akoflow",
		logsRepository:    config.App().Repository.LogsRepository,
		metricsRepository: config.App().Repository.MetricsRepository,

		runtimeRepository: config.App().Repository.RuntimeRepository,

		connector: config.App().Connector.K8sConnector,
	}
}

func (m *MonitorGetLogsActivityService) GetLogs(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) {
	m.handleGetLogsByActivity(wf, wfa)
}

func (m *MonitorGetLogsActivityService) handleGetLogsByActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) {
	fmt.Println("Activity: ", wfa.WorkflowId, wfa.Id)

	nameJob := wfa.GetNameJob()

	runtime, err := m.runtimeRepository.GetByName(wf.GetRuntimeId())
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

	m.retrieveSaveLogsInDatabase(wf, wfa, podName)
}

func (m *MonitorGetLogsActivityService) retrieveSaveLogsInDatabase(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities, podName string) {

	runtime, err := m.runtimeRepository.GetByName(wf.GetRuntimeId())
	if err != nil {
		return
	}

	logs, err := m.connector.Pod(runtime).GetPodLogs(m.namespace, podName)
	if err != nil {
		return
	}

	_ = m.logsRepository.Create(logs_repository.ParamsLogsCreate{
		LogsDatabase: logs_repository.LogsDatabase{
			ActivityId: wfa.Id,
			Logs:       logs,
		},
	})
}
