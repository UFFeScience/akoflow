package apply_job_service

import (
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_activity_dependencies_service"
	"github.com/ovvesley/akoflow/pkg/server/services/make_k8s_job_service"
)

type ApplyJobService struct {
	activityRepository             activity_repository.IActivityRepository
	workflowRepository             workflow_repository.IWorkflowRepository
	connector                      connector.IConnector
	namespace                      string
	getActivityDependenciesService get_activity_dependencies_service.GetActivityDependenciesService
	makeK8sJobService              make_k8s_job_service.MakeK8sJobService
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
			getActivityDependenciesService: params[0].GetActivityDependenciesService,
		}
	}
	return ApplyJobService{
		activityRepository:             activity_repository.New(),
		workflowRepository:             workflow_repository.New(),
		connector:                      connector.New(),
		namespace:                      "akoflow",
		getActivityDependenciesService: get_activity_dependencies_service.New(),
		makeK8sJobService:              make_k8s_job_service.New(),
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

	a.runK8sJob(wf, activity)

	//println("Pod created: ", namePod)

	var _ = a.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusRunning)
	var _ = a.workflowRepository.UpdateStatus(activity.WorkflowId, workflow_repository.StatusRunning)
}

func (a *ApplyJobService) runK8sJob(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) {

	mapWfaDependencies := a.getActivityDependenciesService.GetActivityDependencies(wf.Id)
	dependencies := mapWfaDependencies[wfa.Id]

	println("Dependencies: ", mapWfaDependencies[wfa.Id])

	job, _ := a.makeK8sJobService.
		SetNamespace(a.namespace).
		SetIdWorkflow(wf.Id).
		SetIdWorkflowActivity(wfa.Id).
		SetDependencies(dependencies).
		MakeK8sActivityJob()

	println("Job: ", job.Metadata.Name)

	a.connector.Job().ApplyJob(a.namespace, job)

	podCreated, _ := a.connector.Pod().GetPodByJob(a.namespace, job.Metadata.Name)
	namePod, err := podCreated.GetPodName()

	if err != nil {
		println("Error getting pod name")
		return
	}

	println("Pod created: ", namePod)

}
