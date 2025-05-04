package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/services/get_activity_dependencies_service"
)

type ApplyJobStandaloneService struct {
	namespace string

	activityRepository activity_repository.IActivityRepository
	workflowRepository workflow_repository.IWorkflowRepository
	runtimeRepository  runtime_repository.IRuntimeRepository

	connector connector_k8s.IConnector

	makeK8sJobService              MakeK8sJobService
	getActivityDependenciesService get_activity_dependencies_service.GetActivityDependenciesService
}

func newApplyJobStandaloneService() ApplyJobStandaloneService {
	return ApplyJobStandaloneService{
		namespace: config.App().DefaultNamespace,

		activityRepository: config.App().Repository.ActivityRepository,
		workflowRepository: config.App().Repository.WorkflowRepository,
		runtimeRepository:  config.App().Repository.RuntimeRepository,

		connector: config.App().Connector.K8sConnector,

		makeK8sJobService:              NewMakeK8sJobService(),
		getActivityDependenciesService: get_activity_dependencies_service.New(),
	}
}

func (a *ApplyJobStandaloneService) ApplyStandaloneJob(activityID int) {
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

	_, err = a.runK8sJob(wf, activity)

	var _ = a.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusRunning)
	var _ = a.workflowRepository.UpdateStatus(activity.WorkflowId, workflow_repository.StatusRunning)
}

func (a *ApplyJobStandaloneService) runK8sJob(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) (string, error) {

	mapWfaDependencies := a.getActivityDependenciesService.GetActivityDependencies(wf.Id)
	dependencies := mapWfaDependencies[wfa.Id]

	println("Dependencies: ", mapWfaDependencies[wfa.Id])

	job, _ := a.makeK8sJobService.
		SetNamespace(a.namespace).
		SetWorkflow(wf).
		SetIdWorkflowActivity(wfa.Id).
		UseStandaloneMode().
		SetDependencies(dependencies).
		MakeK8sJob()

	println("Job: ", job.Metadata.Name)

	runtime, err := a.runtimeRepository.GetByName(wf.GetRuntimeId())
	if err != nil {
		return "", err
	}

	a.connector.Job(runtime).ApplyJob(a.namespace, job)

	podCreated, _ := a.connector.Pod(runtime).GetPodByJob(a.namespace, job.Metadata.Name)
	namePod, err := podCreated.GetPodName()

	if err != nil {
		println("Error getting pod name")
		return "", err
	}

	println("Pod created: ", namePod)

	return namePod, nil
}
