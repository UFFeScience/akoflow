package run_activity_in_cluster_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
)

type RunActivityInClusterService struct {
	namespace          string
	workflowRepository *workflow_repository.WorkflowRepository
	activityRepository *activities_repository.ActivityRepository
	channelManager     *channel.Manager
}

func New() *RunActivityInClusterService {
	return &RunActivityInClusterService{
		namespace:          "k8science-cluster-manager",
		workflowRepository: workflow_repository.New(),
		activityRepository: activities_repository.New(),
		channelManager:     channel.GetInstance(),
	}
}

func (r *RunActivityInClusterService) Run(activityID int) {

	activity, err := r.activityRepository.Find(activityID)
	if err != nil {
		println("Activity not found")
		return
	}
	if activity.Status != activities_repository.StatusCreated {
		println("Activity already running")
		return
	}

	println("Running activity: ", activity.Name)

	//time.Sleep(5 * time.Second)
	var _ = r.activityRepository.UpdateStatus(activity.ID, activities_repository.StatusRunning)

}
