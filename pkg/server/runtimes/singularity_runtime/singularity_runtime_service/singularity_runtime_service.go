package singularity_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_singularity"
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

	// runtimeId := wf.GetRuntimeId()

	singularitySystemCall := s.makeSingularityActivity.Handle(wf, wfa)

	s.singularityConnector.RunCommand(singularitySystemCall)

	config.App().Logger.Infof("WORKER: Running singularity command %s", singularitySystemCall)

}
