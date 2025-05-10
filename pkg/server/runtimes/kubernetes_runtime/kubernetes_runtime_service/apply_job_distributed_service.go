package kubernetes_runtime_service

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type ApplyJobDistributedService struct {
	activityRepository activity_repository.IActivityRepository
	workflowRepository workflow_repository.IWorkflowRepository
	runtimeRepository  runtime_repository.IRuntimeRepository

	makeK8sJobService MakeK8sJobService
	connector         connector_k8s.IConnector

	namespace string
}

func newApplyJobDistributedService() ApplyJobDistributedService {
	return ApplyJobDistributedService{
		activityRepository: config.App().Repository.ActivityRepository,
		workflowRepository: config.App().Repository.WorkflowRepository,

		runtimeRepository: config.App().Repository.RuntimeRepository,

		makeK8sJobService: NewMakeK8sJobService(),
		connector:         config.App().Connector.K8sConnector,

		namespace: config.App().DefaultNamespace,
	}
}

func (a *ApplyJobDistributedService) ApplyDistributedJob(activityID int) {
	// do something

	activity, errA := a.activityRepository.Find(activityID)
	wf, errW := a.workflowRepository.Find(activity.WorkflowId)

	if errA != nil {
		println("Activity not found")
		return
	}

	if errW != nil {
		println("Workflow not found")
		return
	}

	if activity.Status != activity_repository.StatusCreated {
		println("Activity already running")
		return
	}

	println("Running activity: ", activity.Name+" in distributed mode by "+fmt.Sprintf("%v", wf.GetId()))

	a.runK8sJob(wf, activity)

	var _ = a.activityRepository.UpdateStatus(activity.Id, activity_repository.StatusRunning)
	var _ = a.workflowRepository.UpdateStatus(activity.WorkflowId, workflow_repository.StatusRunning)

}

func (a *ApplyJobDistributedService) runK8sJob(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) (string, error) {

	job, err := a.makeK8sJobService.
		UseDistributedMode().
		SetNamespace(a.namespace).
		SetWorkflow(wf).
		SetIdWorkflowActivity(wfa.Id).
		MakeK8sJob()

	runtime, err := a.runtimeRepository.GetByName(wfa.GetRuntimeId())
	if err != nil {
		return "", err
	}

	a.connector.Job(runtime).
		ApplyJob(a.namespace, job)

	podCreated, _ := a.connector.Pod(runtime).
		GetPodByJob(a.namespace, job.Metadata.Name)

	namePod, err := podCreated.GetPodName()

	if err != nil {
		println("Error getting pod name")
		return "", err
	}

	println("Pod created: ", namePod)

	return namePod, nil
}
