package run_activity_in_cluster_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
)

type RunActivityInClusterService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activities_repository.IActivityRepository
	channelManager     *channel.Manager
	connector          *connector.Connector
}

type ParamsNewRunActivityInClusterService struct {
	Namespace          string
	WorkflowRepository workflow_repository.IWorkflowRepository
	ActivityRepository activities_repository.IActivityRepository
	ChannelManager     *channel.Manager
	Connector          *connector.Connector
}

func New(params ...ParamsNewRunActivityInClusterService) *RunActivityInClusterService {
	if len(params) > 0 {
		return &RunActivityInClusterService{
			namespace:          params[0].Namespace,
			workflowRepository: params[0].WorkflowRepository,
			activityRepository: params[0].ActivityRepository,
			channelManager:     params[0].ChannelManager,
			connector:          params[0].Connector,
		}
	}

	return &RunActivityInClusterService{
		namespace:          "k8science-cluster-manager",
		workflowRepository: workflow_repository.New(),
		activityRepository: activities_repository.New(),
		channelManager:     channel.GetInstance(),
		connector:          connector.New(),
	}
}

func (r *RunActivityInClusterService) Run(activityID int) {
	resourceOk := r.handleResourceToRunJob(activityID)

	if resourceOk {
		r.handleApplyJob(activityID)
	}

}

func (r *RunActivityInClusterService) handleApplyJob(activityID int) {
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

	var _ = r.activityRepository.UpdateStatus(activity.Id, activities_repository.StatusRunning)
	var _ = r.workflowRepository.UpdateStatus(activity.WorkflowId, workflow_repository.StatusRunning)
}

func (r *RunActivityInClusterService) handleResourceToRunJob(id int) bool {
	activity, err := r.activityRepository.Find(id)
	wf, _ := r.workflowRepository.Find(activity.WorkflowId)

	if err != nil {
		println("Activity not found")
		return false
	}

	namespace := r.handleGetOrCreateNamespace(r.namespace)

	persistent := r.handleGetOrCreatePersistentVolumeClain(wf, namespace)

	return namespace != "" && persistent != ""

}

func (r *RunActivityInClusterService) handleGetOrCreateNamespace(namespace string) string {
	response, err := r.connector.GetNamespace(namespace)

	if err != nil {
		println("Namespace not found")
		return r.handleCreateNamespace(namespace)
	}

	return response.Metadata.Name
}

func (r *RunActivityInClusterService) handleCreateNamespace(namespace string) string {
	ns, err := r.connector.CreateNamespace(namespace)

	if err != nil {
		println("Error creating namespace")
		return ""
	}

	return ns.Metadata.Name

}

func (r *RunActivityInClusterService) handleGetOrCreatePersistentVolumeClain(wf workflow.Workflow, namespace string) string {

	pvc, err := r.connector.GetPersistentVolumeClain(wf.GetVolumeName(), namespace)

	if err != nil {
		println("Persistent volume not found")
		return r.handleCreatePersistentVolumeClain(wf, namespace)
	}

	return pvc.Metadata.Name
}

func (r *RunActivityInClusterService) handleCreatePersistentVolumeClain(wf workflow.Workflow, namespace string) string {
	pv, err := r.connector.CreatePersistentVolumeClain(wf.GetVolumeName(), namespace, wf.Spec.StorageSize, wf.Spec.StorageClassName)

	if err != nil {
		println("Error creating persistent volume")
		return ""
	}

	if pv.Metadata.Name == "" {
		println("Error creating persistent volume")
		return ""
	}

	return pv.Metadata.Name
}
