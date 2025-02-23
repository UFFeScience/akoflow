package sdumont_runtime_service

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_sdumont"
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

	return output

}
