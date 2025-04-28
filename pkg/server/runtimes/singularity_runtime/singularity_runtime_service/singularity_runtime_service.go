package singularity_runtime_service

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_singularity"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/logs_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/metrics_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type SingularityRuntimeService struct {
	activityRepository activity_repository.IActivityRepository
	workflowRepository workflow_repository.IWorkflowRepository
	metricsRepository  metrics_repository.IMetricsRepository
	logsRepository     logs_repository.ILogsRepository

	makeSingularityActivity MakeSingularityActivityService
	singularityConnector    connector_singularity.IConnectorSingularity
}

func NewSingularityRuntimeService() SingularityRuntimeService {
	return SingularityRuntimeService{
		activityRepository: config.App().Repository.ActivityRepository,
		workflowRepository: config.App().Repository.WorkflowRepository,
		metricsRepository:  config.App().Repository.MetricsRepository,
		logsRepository:     config.App().Repository.LogsRepository,

		makeSingularityActivity: NewMakeSingularityActivityService(),
		singularityConnector:    config.App().Connector.SingularityConnector,
	}
}

func (s *SingularityRuntimeService) ApplyJob(workflowID int, activityID int) {
	wfa, err := s.activityRepository.Find(activityID)
	wf, _ := s.workflowRepository.Find(wfa.WorkflowId)

	if err != nil {
		config.App().Logger.Infof("WORKER: Activity not found %d", activityID)
		return
	}

	singularitySystemCall := s.makeSingularityActivity.Handle(wf, wfa)

	pid, _ := s.singularityConnector.RunCommand(singularitySystemCall)

	fmt.Println("PID: ", pid)

	err = s.workflowRepository.UpdateStatus(wfa.WorkflowId, workflow_repository.StatusRunning)

	if err != nil {
		config.App().Logger.Infof("WORKER: Error updating workflow status %d", wfa.WorkflowId)
		return
	}

	_ = s.activityRepository.UpdateStatus(wfa.Id, activity_repository.StatusRunning)
	err = s.activityRepository.UpdateProcID(wfa.Id, pid)

	if err != nil {
		config.App().Logger.Infof("WORKER: Error updating activity status %d", activityID)
		return
	}

	config.App().Logger.Infof("WORKER: Running singularity command %s", singularitySystemCall)

}

func (s *SingularityRuntimeService) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) {
	for _, activity := range workflow.Spec.Activities {
		if activity.Status != activity_repository.StatusRunning {
			continue
		}

		pid := activity.ProcId

		akfMonitorBashScript, err := NewAkfMonitorSingularity().
			SetWorkflow(workflow).
			SetWorkflowActivity(activity).
			GetScript()

		if err != nil {
			config.App().Logger.Infof("WORKER: Error creating akf monitor script %s", pid)
			return
		}

		commandBase64 := base64.StdEncoding.EncodeToString([]byte(akfMonitorBashScript))
		commandFinal := "echo " + commandBase64 + " | base64 -d | sh"

		outputCommand, _ := s.singularityConnector.RunCommandWithOutput(commandFinal)

		if s.ProcessCompleted(outputCommand) {
			s.handleProcessCompleted(workflow, activity, pid, outputCommand)
			return
		}

		s.handleProcessRunning(workflow, activity, pid, outputCommand)

	}
}

func (s *SingularityRuntimeService) handleProcessCompleted(_ workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities, __ string, ___ string) {
	s.activityRepository.UpdateStatus(workflowActivity.GetId(), activity_repository.StatusFinished)
}

func (s *SingularityRuntimeService) handleProcessRunning(_ workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities, __ string, outputCommand string) {
	totalCPU, totalMEM, err := s.ExtractMetrics(outputCommand)

	activityID := workflowActivity.GetId()

	if err != nil {
		config.App().Logger.Infof("WORKER: Error extracting metrics %d", activityID)
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	err = s.metricsRepository.Create(metrics_repository.ParamsMetricsCreate{
		MetricsDatabase: metrics_repository.MetricsDatabase{
			ActivityId: activityID,
			Cpu:        totalCPU,
			Memory:     totalMEM,
			Window:     "1s",
			Timestamp:  timestamp,
		},
	})

	if err != nil {
		config.App().Logger.Infof("WORKER: Error updating metrics %d", activityID)
		return
	}

	logsOutput, logsErr, err := s.ExtractLogs(outputCommand)

	if err != nil {
		config.App().Logger.Infof("WORKER: Error extracting logs %d", activityID)
		return
	}

	if logsOutput != "" {
		s.logsRepository.Create(logs_repository.ParamsLogsCreate{
			LogsDatabase: logs_repository.LogsDatabase{
				ActivityId: activityID,
				Logs:       logsOutput,
			},
		})

	}

	if logsErr != "" {
		s.logsRepository.Create(logs_repository.ParamsLogsCreate{
			LogsDatabase: logs_repository.LogsDatabase{
				ActivityId: activityID,
				Logs:       logsErr,
			},
		})
	}

}

func (s *SingularityRuntimeService) ExtractLogs(outputCommand string) (string, string, error) {
	reOutput := regexp.MustCompile(`(?s)##START_LOG_OUTPUT##(.*)##END_LOG_OUTPUT##`)
	reError := regexp.MustCompile(`(?s)##START_LOG_ERROR##(.*)##END_LOG_ERROR##`)

	matchOutput := reOutput.FindStringSubmatch(outputCommand)
	matchError := reError.FindStringSubmatch(outputCommand)

	var logsOutput string
	var logsErr string

	if len(matchOutput) > 1 {
		logsOutput = strings.TrimSpace(matchOutput[1])
	}

	if len(matchError) > 1 {
		logsErr = strings.TrimSpace(matchError[1])
	}

	return logsOutput, logsErr, nil
}

func (s *SingularityRuntimeService) ProcessCompleted(outputCommand string) bool {
	if strings.Contains(outputCommand, "#NO_PROCESS_FOUND") {
		config.App().Logger.Infof("WORKER: No process found in the output")
		return true
	}
	return false
}

func (s *SingularityRuntimeService) ExtractMetrics(metrics string) (string, string, error) {
	var re = regexp.MustCompile(`(?s)TOTAL_CPU=\((.*?)%\).*?TOTAL_MEM=\((.*?)%`)
	var str = metrics
	matches := re.FindStringSubmatch(str)

	if len(matches) == 0 {
		return "", "", fmt.Errorf("no metrics found")
	}

	totalCpu := matches[1]
	totalMem := matches[2]

	return totalCpu, totalMem, nil
}
