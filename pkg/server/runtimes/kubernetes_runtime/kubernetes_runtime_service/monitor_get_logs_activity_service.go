package kubernetes_runtime_service

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/metrics_repository"
)

type MonitorGetLogsActivityService struct {
	namespace         string
	logsRepository    logs_repository.ILogsRepository
	metricsRepository metrics_repository.IMetricsRepository
	connector         connector_k8s.IConnector
}

func NewMonitorGetLogsActivityService() *MonitorGetLogsActivityService {
	return &MonitorGetLogsActivityService{
		namespace:         "akoflow",
		logsRepository:    config.App().Repository.LogsRepository,
		metricsRepository: config.App().Repository.MetricsRepository,
		connector:         config.App().Connector.K8sConnector,
	}
}

func (m *MonitorGetLogsActivityService) GetLogs(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) {
	m.handleGetLogsByActivity(wfa)
}

func (m *MonitorGetLogsActivityService) handleGetLogsByActivity(wfa workflow_activity_entity.WorkflowActivities) {
	fmt.Println("Activity: ", wfa.WorkflowId, wfa.Id)

	nameJob := wfa.GetNameJob()

	job, err := m.connector.Pod().GetPodByJob(m.namespace, nameJob)
	if err != nil {
		return
	}

	podName, err := job.GetPodName()
	if err != nil {
		return
	}

	m.retrieveSaveLogsInDatabase(wfa, podName)
}

func (m *MonitorGetLogsActivityService) retrieveSaveLogsInDatabase(wfa workflow_activity_entity.WorkflowActivities, podName string) {
	logs, err := m.connector.Pod().GetPodLogs(m.namespace, podName)
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
