package sdumont_runtime_service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_sdumont"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/singularity_runtime/singularity_runtime_service"
)

type SDumontRuntimeService struct {
	makeSingularityActivity   singularity_runtime_service.MakeSingularityActivityService
	makeSBatchSDumontActivity MakeSBatchSDumontActivityService

	activityRepository activity_repository.IActivityRepository
	workflowRepository workflow_repository.IWorkflowRepository
	runtimeRepository  runtime_repository.IRuntimeRepository

	connectorSDumont connector_sdumont.IConnectorSDumont
}

func New() *SDumontRuntimeService {
	return &SDumontRuntimeService{
		makeSingularityActivity: singularity_runtime_service.NewMakeSingularityActivityService(),

		activityRepository: config.App().Repository.ActivityRepository,
		workflowRepository: config.App().Repository.WorkflowRepository,
		runtimeRepository:  config.App().Repository.RuntimeRepository,

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

	s.updateWorkflowAndActivityStatus(wfa)

	s.syncWorkflowVolumes(wf)
}

func (s *SDumontRuntimeService) updateWorkflowAndActivityStatus(wfa workflow_activity_entity.WorkflowActivities) {
	_ = s.workflowRepository.UpdateStatus(wfa.WorkflowId, workflow_repository.StatusRunning)
	_ = s.activityRepository.UpdateStatus(wfa.Id, activity_repository.StatusRunning)
}

func (s *SDumontRuntimeService) syncWorkflowVolumes(wf workflow_entity.Workflow) {
	volumes := wf.GetVolumes()
	commands := []string{}

	runtime, err := s.runtimeRepository.GetByName(wf.GetRuntimeId()[0])
	if err != nil {
		config.App().Logger.Infof("WORKER: Error getting runtime from database.")
		return
	}

	for _, volume := range volumes {
		// Sync local to remote
		command1 := fmt.Sprintf("sshpass -p '%s' ssh -o StrictHostKeyChecking=no %s@%s 'mkdir -p %s'",
			runtime.GetCurrentRuntimeMetadata("PASSWORD"),
			runtime.GetCurrentRuntimeMetadata("USER"),
			runtime.GetCurrentRuntimeMetadata("HOST_CLUSTER"),
			volume.GetRemotePath(),
		)

		command2 := fmt.Sprintf("sshpass -p '%s' rsync -ah --progress %s %s@%s:%s",
			runtime.GetCurrentRuntimeMetadata("PASSWORD"),
			volume.GetLocalPath(),
			runtime.GetCurrentRuntimeMetadata("USER"),
			runtime.GetCurrentRuntimeMetadata("HOST_CLUSTER"),
			volume.GetRemotePath(),
		)

		// Sync remote to local
		command3 := fmt.Sprintf("sshpass -p '%s' rsync -ah --progress %s@%s:%s %s",
			runtime.GetCurrentRuntimeMetadata("PASSWORD"),
			runtime.GetCurrentRuntimeMetadata("USER"),
			runtime.GetCurrentRuntimeMetadata("HOST_CLUSTER"),
			volume.GetRemotePath(),
			volume.GetLocalPath(),
		)

		fullCommands := fmt.Sprintf("%s && %s && %s", command1, command2, command3)

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

func (s *SDumontRuntimeService) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool {
	config.App().Logger.Infof("WORKER: Verify activities was finished in SDumont Runtime")

	for _, activity := range workflow.Spec.Activities {
		s.handleVerifyActivityWasFinished(activity, workflow)
	}
	return true
}

func (s *SDumontRuntimeService) handleVerifyActivityWasFinished(activity workflow_activity_entity.WorkflowActivities, wf workflow_entity.Workflow) int {
	println("Verifying activity: ", activity.Name, " with id: ", activity.Id)

	wfaDatabase, _ := s.activityRepository.Find(activity.Id)

	if wfaDatabase.Status == activity_repository.StatusFinished {
		return activity_repository.StatusFinished
	}

	if wfaDatabase.Status == activity_repository.StatusCreated {
		return activity_repository.StatusCreated
	}

	command := fmt.Sprintf(" sacct -j %s  --format=JobID,JobName,Partition,Account,AllocCPUs,State,ExitCode --noheader | grep akoflow", wfaDatabase.GetProcId())

	output, _ := s.connectorSDumont.RunCommandWithOutputRemote(command)

	saactResponse, err := s.extractSacctJobID(output)

	if err != nil {
		config.App().Logger.Infof("WORKER: Error extracting job ID %s", strings.TrimSpace(wfaDatabase.GetProcId()))
		return activity_repository.StatusRunning
	}

	if saactResponse.State == "COMPLETED" {
		config.App().Logger.Infof("WORKER: Activity %d finished", activity.Id)
		_ = s.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusFinished)
		_ = s.workflowRepository.UpdateStatus(wf.GetId(), workflow_repository.StatusFinished)
		s.syncWorkflowVolumes(wf)
		return activity_repository.StatusFinished
	}

	if saactResponse.State == "FAILED" {
		config.App().Logger.Infof("WORKER: Activity %d failed", activity.Id)
		_ = s.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusFinished)
		_ = s.workflowRepository.UpdateStatus(wf.GetId(), workflow_repository.StatusFinished)
		s.syncWorkflowVolumes(wf)
		return activity_repository.StatusFinished
	}

	if saactResponse.State == "CANCELLED+" || saactResponse.State == "CANCELLED" {
		config.App().Logger.Infof("WORKER: Activity %d cancelled", activity.Id)
		_ = s.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusFinished)
		_ = s.workflowRepository.UpdateStatus(wf.GetId(), workflow_repository.StatusFinished)
		s.syncWorkflowVolumes(wf)
		return activity_repository.StatusFinished
	}

	if saactResponse.State == "RUNNING" {
		config.App().Logger.Infof("WORKER: Activity %d running", activity.Id)
		_ = s.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusRunning)
		return activity_repository.StatusRunning
	}

	if saactResponse.State == "PENDING" {
		config.App().Logger.Infof("WORKER: Activity %d pending", activity.Id)
		_ = s.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusRunning)
		return activity_repository.StatusRunning
	}

	return activity_repository.StatusRunning

}

type SaactResponse struct {
	JobID     string `json:"JobID"`
	JobName   string `json:"JobName"`
	Partition string `json:"Partition"`
	Account   string `json:"Account"`
	AllocCPUs string `json:"AllocCPUs"`
	State     string `json:"State"`
	ExitCode  string `json:"ExitCode"`
}

func (s *SDumontRuntimeService) extractSacctJobID(outputCommand string) (SaactResponse, error) {
	reOutput := regexp.MustCompile(`(?m)(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\d+)\s+(\S+)\s+(\d+:\d+)`)
	match := reOutput.FindStringSubmatch(outputCommand)

	if len(match) == 0 {
		return SaactResponse{}, fmt.Errorf("no match found")
	}

	if len(match) < 8 {
		return SaactResponse{}, fmt.Errorf("invalid output format")
	}

	return SaactResponse{
		JobID:     match[1],
		JobName:   match[2],
		Partition: match[3],
		Account:   match[4],
		AllocCPUs: match[5],
		State:     match[6],
		ExitCode:  match[7],
	}, nil
}

func (s *SDumontRuntimeService) HealthCheck(runtimeName string) bool {
	config.App().Logger.Infof("WORKER: Health check SDumont Runtime")

	connected, err := s.connectorSDumont.IsVPNConnected()
	if err != nil {
		config.App().Logger.Error("WORKER: Error checking VPN connection to SDumont Runtime.")
		return false
	}
	if !connected {
		config.App().Logger.Error("WORKER: VPN is not connected to SDumont Runtime.")
		return false
	}

	runtime, err := s.runtimeRepository.GetByName(runtimeName)

	if err != nil {
		config.App().Logger.Error("WORKER: Error getting runtime from database.")
		return false
	}

	if runtime == nil {
		config.App().Logger.Error("WORKER: Runtime not found in database.")
		return false
	}

	command := fmt.Sprintf("sinfo -p %s", runtime.GetCurrentRuntimeMetadata("QUEUE"))
	output, err := s.connectorSDumont.SetRuntime(*runtime).RunCommandWithOutputRemote(command)

	if err != nil {
		runtime.Status = runtime_repository.STATUS_NOT_READY
		config.App().Logger.Error("WORKER: Error running command in SDumont Runtime.")
		return false
	}

	if strings.Contains(output, "No nodes available") {
		s.runtimeRepository.UpdateStatus(runtime, runtime_repository.STATUS_NOT_READY)
		config.App().Logger.Error("WORKER: No nodes available in SDumont Runtime.")
		return false
	}

	s.runtimeRepository.UpdateStatus(runtime, runtime_repository.STATUS_READY)

	return true

}
