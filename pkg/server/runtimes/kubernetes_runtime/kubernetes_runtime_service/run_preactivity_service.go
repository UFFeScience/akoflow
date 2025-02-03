package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_activity_dependencies_service"
)

type RunPreactivityService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository

	connector connector.IConnector

	makeK8sJobService              MakeK8sJobService
	getActivityDependenciesService get_activity_dependencies_service.GetActivityDependenciesService
}

func NewRunPreactivityService() RunPreactivityService {
	return RunPreactivityService{
		namespace:          config.App().DefaultNamespace,
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,

		connector: config.App().Connector.K8sConnector,

		makeK8sJobService:              NewMakeK8sJobService(),
		getActivityDependenciesService: get_activity_dependencies_service.New(),
	}
}

func (r *RunPreactivityService) Run(activityID int) (resourceOk bool, err error) {

	wfa, err := r.activityRepository.Find(activityID)
	wf, _ := r.workflowRepository.Find(wfa.WorkflowId)

	if !wfa.HasDependencies() {
		return true, nil
	}

	wfpreActivity, err := r.activityRepository.FindPreActivity(activityID)

	if err != nil {
		return false, err
	}

	if wfpreActivity.Status == activity_repository.StatusRunning {
		return false, nil
	}

	if wfpreActivity.Status == activity_repository.StatusFinished {
		return true, nil
	}

	r.runJobPreActivity(wf, wfa, wfpreActivity)

	return false, nil
}

func (r *RunPreactivityService) runJobPreActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities, wfpreActivity workflow_activity_entity.WorkflowPreActivityDatabase) {

	wfpreActivity.Status = activity_repository.StatusRunning

	mapWfaDependencies := r.getActivityDependenciesService.GetActivityDependenciesByActivity(wfa.WorkflowId, wfa.Id)
	dependencies := mapWfaDependencies[wfa.Id]

	job, _ := r.makeK8sJobService.
		SetNamespace(r.namespace).
		SetIdWorkflow(wf.Id).
		SetIdWorkflowActivity(wfa.Id).
		SetDependencies(dependencies).
		UsePreactivityMode().
		MakeK8sJob()

	r.connector.Job().ApplyJob(r.namespace, job)

	resourceJobK8sBase64 := job.GetBase64Jobs()
	wfpreActivity.ResourceK8sBase64 = &resourceJobK8sBase64
	r.activityRepository.UpdatePreActivity(wfpreActivity.Id, wfpreActivity)

}
