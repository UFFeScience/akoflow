package run_activity_in_cluster_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
)

type RunActivityInClusterService struct {
	namespace          string
	workflowRepository *workflow_repository.WorkflowRepository
	activityRepository *activities_repository.ActivityRepository
	channelManager     *channel.Manager
	connector          *connector.Connector
}

func New() *RunActivityInClusterService {
	return &RunActivityInClusterService{
		namespace:          "k8science-cluster-manager",
		workflowRepository: workflow_repository.New(),
		activityRepository: activities_repository.New(),
		channelManager:     channel.GetInstance(),
		connector:          connector.New(),
	}
}

func (r *RunActivityInClusterService) Run(activityID int) {

	activity, err := r.activityRepository.Find(activityID)
	wf, _ := r.workflowRepository.Find(activity.WorkflowId)

	if err != nil {
		println("Activity not found")
		return
	}
	if activity.Status != activities_repository.StatusCreated {
		println("Activity already running")
		return
	}

	println("Running activity: ", activity.Name)

	k8sJob := activity.MakeResourceK8s(wf)
	r.connector.ApplyJob(r.namespace, k8sJob)

	podCreated, _ := r.connector.GetPodByJob(r.namespace, activity.GetName())
	namePod, err := podCreated.GetPodName()

	if err != nil {
		println("Error getting pod name")
		return
	}

	println("Pod created: ", namePod)

	var _ = r.activityRepository.UpdateStatus(activity.ID, activities_repository.StatusRunning)
	var _ = r.workflowRepository.UpdateStatus(activity.WorkflowId, workflow_repository.StatusRunning)

}
