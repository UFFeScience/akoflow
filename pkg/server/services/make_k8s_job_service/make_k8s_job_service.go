package make_k8s_job_service

import (
	"errors"
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
)

var ImagePreActivity = "ovvesley/akoflow-preactivity:latest"

type MakeK8sJobService struct {
	namespace          string
	dependencies       []workflow_activity_entity.WorkflowActivities
	idWorkflow         int
	idWorkflowActivity int
	workflow           workflow_entity.Workflow

	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository

	makeK8sActivityService MakeK8sActivityService

	makeK8sActivityDistributedService MakeK8sActivityDistributedService
	makeK8sActivityStandaloneService  MakeK8sActivityStandaloneService
	makeK8sActivityPreactivityService MakeK8sActivityPreactivityService
}

// New creates a new MakeK8sJobService.
func New() MakeK8sJobService {
	return MakeK8sJobService{
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,

		makeK8sActivityService: newMakeK8sActivityService(),

		makeK8sActivityDistributedService: newMakeK8sActivityDistributedService(),
		makeK8sActivityStandaloneService:  newMakeK8sActivityStandaloneService(),
		makeK8sActivityPreactivityService: newMakeK8sActivityPreactivityService(),
	}
}

// SetNamespace sets the namespace where the k8s job will be created.
func (m *MakeK8sJobService) SetNamespace(namespace string) *MakeK8sJobService {
	m.namespace = namespace
	return m
}

// SetWorkflow sets the workflow that will be used to make the k8s job.
func (m *MakeK8sJobService) SetWorkflow(workflow workflow_entity.Workflow) *MakeK8sJobService {
	m.workflow = workflow
	m.idWorkflow = workflow.Id
	return m
}

// SetDependencies sets the dependencies of the activity.
func (m *MakeK8sJobService) SetDependencies(dependencies []workflow_activity_entity.WorkflowActivities) *MakeK8sJobService {
	m.dependencies = dependencies
	return m
}

// SetIdWorkflow sets the id of the workflow that will be used to make the k8s job.
func (m *MakeK8sJobService) SetIdWorkflow(idWorkflow int) *MakeK8sJobService {
	m.idWorkflow = idWorkflow
	return m
}

// SetIdWorkflowActivity sets the id of the activity that will be used to make the k8s job.
func (m *MakeK8sJobService) SetIdWorkflowActivity(idWorkflowActivity int) *MakeK8sJobService {
	m.idWorkflowActivity = idWorkflowActivity
	return m
}

// getDependencies returns the dependencies of the activity.
func (m *MakeK8sJobService) getDependencies() []workflow_activity_entity.WorkflowActivities {
	return m.dependencies
}

// getIdWorkflow returns the id of the workflow that will be used to make the k8s job.
func (m *MakeK8sJobService) getIdWorkflow() int {
	return m.idWorkflow
}

// getIdWorkflowActivity returns the id of the activity that will be used to make the k8s job.
func (m *MakeK8sJobService) getIdWorkflowActivity() int {
	return m.idWorkflowActivity
}

func (m *MakeK8sJobService) MakeK8sActivityJob() (k8s_job_entity.K8sJob, error) {
	// Check if the parameters are valid to make a k8s job.
}

func (m *MakeK8sJobService) makeK8sActivityStandaloneJob() (k8s_job_entity.K8sJob, error) {
	if !m.isValidate() {
		return k8s_job_entity.K8sJob{}, errors.New("invalid parameters to make k8s job:: namespace, persistentVolumeClaim, dependencies, idWorkflow, idWorkflowActivity are required")
	}

	workflow, _ := m.workflowRepository.Find(m.getIdWorkflow())
	activity, _ := m.activityRepository.Find(m.getIdWorkflowActivity())

	container := m.makeContainerActivity(workflow, activity)
	volumes := m.makeVolumesActivity(workflow, activity)

	k8sJob := k8s_job_entity.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8s_job_entity.K8sJobMetadata{
			Name: activity.GetNameJob(),
		},
		Spec: k8s_job_entity.K8sJobSpec{
			BackoffLimit: 0,
			Template: k8s_job_entity.K8sJobTemplate{
				Spec: k8s_job_entity.K8sJobSpecTemplate{
					Containers:    []k8s_job_entity.K8sJobContainer{container},
					RestartPolicy: "Never",
					Volumes:       volumes,
				},
			},
		},
	}

	nodeSelector := m.makeNodeSelector(workflow, activity)
	if nodeSelector != nil {
		k8sJob.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	return k8sJob, nil

}

func (m *MakeK8sJobService) makeK8sActivityPreActivityJob() (k8s_job_entity.K8sJob, error) {
	if !m.isValidate() {
		return k8s_job_entity.K8sJob{}, errors.New("invalid parameters to make k8s job:: namespace, persistentVolumeClaim, dependencies, idWorkflow, idWorkflowActivity are required")
	}

	workflow, _ := m.workflowRepository.Find(m.getIdWorkflow())
	activity, _ := m.activityRepository.Find(m.getIdWorkflowActivity())
	preActivity, _ := m.activityRepository.FindPreActivity(m.getIdWorkflowActivity())

	volumes := m.makeVolumesPreActivity(workflow, activity)
	container := m.makeContainerPreActivity(workflow, activity)

	k8sJob := k8s_job_entity.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8s_job_entity.K8sJobMetadata{
			Name: activity.GetPreActivityName(),
		},
		Spec: k8s_job_entity.K8sJobSpec{
			BackoffLimit: 0,
			Template: k8s_job_entity.K8sJobTemplate{
				Spec: k8s_job_entity.K8sJobSpecTemplate{
					Containers:    []k8s_job_entity.K8sJobContainer{container},
					RestartPolicy: "Never",
					Volumes:       volumes,
				},
			},
		},
	}

	nodeSelector := m.makeNodeSelector(workflow, activity)
	if nodeSelector != nil {
		k8sJob.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	println("Running pre activity: ", preActivity.Name)
	println("Workflow: ", workflow.Name)
	println("Activity: ", activity.Name)

	return k8sJob, nil

}

func (m *MakeK8sJobService) makeK8sActivityDistributedJob() (k8s_job_entity.K8sJob, error) {

}
