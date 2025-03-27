package sdumont_runtime_service

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_sdumont"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/singularity_runtime/singularity_runtime_service"
)

type SDumontRuntimeService struct {
	makeSingularityActivity   singularity_runtime_service.MakeSingularityActivityService
	makeSBatchSDumontActivity MakeSBatchSDumontActivityService

	activityRepository activity_repository.IActivityRepository
	workflowRepository workflow_repository.IWorkflowRepository

	connectorSDumont connector_sdumont.IConnectorSDumont
}

func New() *SDumontRuntimeService {
	return &SDumontRuntimeService{
		makeSingularityActivity: singularity_runtime_service.NewMakeSingularityActivityService(),

		activityRepository: config.App().Repository.ActivityRepository,
		workflowRepository: config.App().Repository.WorkflowRepository,

		connectorSDumont: config.App().Connector.SDumontConnector,
	}
}

func (s *SDumontRuntimeService) ApplyJob(workflowID int, activityID int) string {
	wfa, err := s.activityRepository.Find(activityID)
	wf, _ := s.workflowRepository.Find(wfa.WorkflowId)

	if err != nil {
		config.App().Logger.Infof("WORKER: Activity not found %d", activityID)
		return ""
	}

	if wfa.Status == activity_repository.StatusRunning {
		config.App().Logger.Infof("WORKER: Activity already running %d", activityID)
		return ""
	}

	if wf.Status == activity_repository.StatusCreated {
		config.App().Logger.Infof("WORKER: Initial activity. Setup Data and Environment.")
		s.applyWorkflowInRuntime(wf, wfa)
	}

	singularitySystemCall := s.makeSingularityActivity.MakeContainerCommandActivityToSDumont(wf, wfa)
	sBatchSDumontSystemCall := s.makeSBatchSDumontActivity.
		SetSingularityCommand(singularitySystemCall).
		Handle(wf, wfa)

	fmt.Println("PID: ", singularitySystemCall, sBatchSDumontSystemCall)

	connected, err := s.connectorSDumont.IsVPNConnected()

	if err != nil {
		config.App().Logger.Error("WORKER: Error checking VPN connection to SDumont Runtime.")
		return ""
	}

	if !connected {
		config.App().Logger.Error("WORKER: VPN is not connected to SDumont Runtime. Continue to the next activity.")
		return ""
	}

	output, _ := s.connectorSDumont.RunCommandWithOutputRemote(sBatchSDumontSystemCall)

	pid, err := s.extractJobID(output)

	fmt.Println("PID: ", pid)

	err = s.workflowRepository.UpdateStatus(wfa.WorkflowId, workflow_repository.StatusRunning)

	if err != nil {
		config.App().Logger.Infof("WORKER: Error updating workflow status %d", wfa.WorkflowId)
		return ""
	}

	_ = s.activityRepository.UpdateStatus(wfa.Id, activity_repository.StatusRunning)
	err = s.activityRepository.UpdateProcID(wfa.Id, pid)

	if err != nil {
		config.App().Logger.Infof("WORKER: Error updating activity status %d", activityID)
		return ""
	}

	config.App().Logger.Infof("WORKER: Running singularity command %s", singularitySystemCall)

	return output

}

func (s *SDumontRuntimeService) applyWorkflowInRuntime(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) {
	config.App().Logger.Infof("WORKER: Apply workflow in SDumont Runtime")

	_ = s.workflowRepository.UpdateStatus(wfa.WorkflowId, workflow_repository.StatusRunning)
	_ = s.activityRepository.UpdateStatus(wfa.Id, activity_repository.StatusRunning)

	volumes := wf.GetVolumes()

	commands := []string{}

	// TODO: Refactor this to a better way disacopling the commands in a make service
	for _, volume := range volumes {
		command1 := fmt.Sprintf("sshpass -p '%s' ssh -o StrictHostKeyChecking=no %s@%s 'mkdir -p %s'",
			os.Getenv("SDUMONT_PASSWORD"),
			os.Getenv("SDUMONT_USER"),
			os.Getenv("SDUMONT_HOST_CLUSTER"),
			volume.GetRemotePath(),
		)

		command2 := fmt.Sprintf("sshpass -p '%s' rsync -ah --progress %s %s@%s:%s",
			os.Getenv("SDUMONT_PASSWORD"),
			volume.GetLocalPath(),
			os.Getenv("SDUMONT_USER"),
			os.Getenv("SDUMONT_HOST_CLUSTER"),
			volume.GetRemotePath(),
		)

		fullCommands := fmt.Sprintf("%s && %s", command1, command2)

		commands = append(commands, fullCommands)

	}

	s.connectorSDumont.ExecuteMultiplesCommand(commands)

}

func (s *SDumontRuntimeService) extractJobID(outputCommand string) (string, error) {
	reOutput := regexp.MustCompile(`(?m)(\d+)`)

	matchOutput := reOutput.FindStringSubmatch(outputCommand)

	var logsOutput string

	if len(matchOutput) > 1 {
		logsOutput = strings.TrimSpace(matchOutput[1])
	}

	return logsOutput, nil
}
