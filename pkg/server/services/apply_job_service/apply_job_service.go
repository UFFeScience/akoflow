package apply_job_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/connector"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/services/get_activity_dependencies_service"
)

type ApplyJobService struct {
	activityRepository             activity_repository.IActivityRepository
	workflowRepository             workflow_repository.IWorkflowRepository
	connector                      connector.IConnector
	namespace                      string
	GetActivityDependenciesService get_activity_dependencies_service.GetActivityDependenciesService
}

type ParamsNewApplyJobService struct {
	ActivityRepository             activity_repository.IActivityRepository
	WorkflowRepository             workflow_repository.IWorkflowRepository
	Namespace                      string
	GetActivityDependenciesService get_activity_dependencies_service.GetActivityDependenciesService
}

func New(params ...ParamsNewApplyJobService) ApplyJobService {
	if len(params) > 0 {
		return ApplyJobService{
			activityRepository:             params[0].ActivityRepository,
			workflowRepository:             params[0].WorkflowRepository,
			connector:                      connector.New(),
			namespace:                      params[0].Namespace,
			GetActivityDependenciesService: params[0].GetActivityDependenciesService,
		}
	}
	return ApplyJobService{
		activityRepository:             activity_repository.New(),
		workflowRepository:             workflow_repository.New(),
		connector:                      connector.New(),
		namespace:                      "scik8sflow",
		GetActivityDependenciesService: get_activity_dependencies_service.New(),
	}
}

func (a *ApplyJobService) ApplyJob(activityID int) {
	a.handleApplyJob(activityID)
}

func (a *ApplyJobService) handleApplyJob(activityID int) {
	activity, err := a.activityRepository.Find(activityID)
	wf, _ := a.workflowRepository.Find(activity.WorkflowId)

	if err != nil {
		println("Activity not found")
		return
	}
	if activity.Status != activity_repository.StatusCreated {
		println("Activity already running")
		return
	}

	println("Running activity: ", activity.Name)

	activities := a.GetActivityDependenciesService.GetActivityDependencies(wf.Id)
	println("Activities: ", len(activities))

	if err != nil {
		println("Error getting pod name")
		return
	}

	//println("Pod created: ", namePod)

	var _ = a.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusRunning)
	var _ = a.workflowRepository.UpdateStatus(activity.WorkflowId, workflow_repository.StatusRunning)
}
