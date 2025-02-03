package run_activity_in_cluster_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/runtimes"
)

type RunActivityInClusterService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func New() *RunActivityInClusterService {
	return &RunActivityInClusterService{
		namespace:          config.App().DefaultNamespace,
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
	}
}

func (r *RunActivityInClusterService) Run(activityID int) {

	wfa, err := r.activityRepository.Find(activityID)
	wf, _ := r.workflowRepository.Find(wfa.WorkflowId)

	if err != nil {
		config.App().Logger.Infof("WORKER: Activity not found %d", activityID)
		return
	}

	runtimeId := "k8s"

	workflowId := wf.GetId()
	workflowActivityId := wfa.GetId()

	runtimes.
		GetRuntimeInstance(runtimeId).
		ApplyJob(workflowId, workflowActivityId)

	config.App().Logger.Infof("WORKER: Activity %d started", activityID)

}
