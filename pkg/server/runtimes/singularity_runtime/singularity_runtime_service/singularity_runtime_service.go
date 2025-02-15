package singularity_runtime_service

import (
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_singularity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
)

type SingularityRuntimeService struct {
	activityRepository      activity_repository.IActivityRepository
	workflowRepository      workflow_repository.IWorkflowRepository
	makeSingularityActivity MakeSingularityActivityService
	singularityConnector    connector_singularity.IConnectorSingularity
}

func NewSingularityRuntimeService() SingularityRuntimeService {
	return SingularityRuntimeService{
		activityRepository: config.App().Repository.ActivityRepository,
		workflowRepository: config.App().Repository.WorkflowRepository,

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

	err = s.activityRepository.UpdateStatus(wfa.Id, activity_repository.StatusRunning)
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
			SetPid(pid).
			GetScript()

		if err != nil {
			config.App().Logger.Infof("WORKER: Error creating akf monitor script %s", pid)
			return
		}

		commandBase64 := base64.StdEncoding.EncodeToString([]byte(akfMonitorBashScript))
		commandFinal := "echo " + commandBase64 + " | base64 -d | sh"

		outputCommand, _ := s.singularityConnector.RunCommandWithOutput(commandFinal)

		totalCPU, totalMEM, err := s.ExtractMetrics(outputCommand)

		if err != nil {
			config.App().Logger.Infof("WORKER: Error extracting metrics %s", outputCommand)
			return
		}

		fmt.Println("Total CPU: ", totalCPU)
		fmt.Println("Total MEM: ", totalMEM)

		fmt.Println("Metrics: ", outputCommand)

	}
}

func (s *SingularityRuntimeService) ExtractMetrics(metrics string) (string, string, error) {
	re := regexp.MustCompile(`TOTAL_CPU=\((.*?)%\).*?TOTAL_MEM=\((.*?)%`)
	matches := re.FindStringSubmatch(metrics)

	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not find metrics in the provided string")
	}

	totalCPU := matches[1]
	totalMEM := matches[2]

	return totalCPU, totalMEM, nil
}
