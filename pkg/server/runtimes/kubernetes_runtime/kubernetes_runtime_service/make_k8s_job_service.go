package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

var ImagePreActivity = "ovvesley/akoflow-preactivity:latest"

var MODE_STANDALONE = "standalone"
var MODE_DISTRIBUTED = "distributed"
var MODE_PREACTIVITY = "preactivity"

type IMakeK8sJobService interface {
	Handle(service MakeK8sJobService) (k8s_job_entity.K8sJob, error)
}

type MakeK8sJobService struct {
	namespace          string
	dependencies       []workflow_activity_entity.WorkflowActivities
	idWorkflow         int
	idWorkflowActivity int
	workflow           workflow_entity.Workflow

	mode string

	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository

	makeK8sActivityService MakeK8sActivityService

	makeK8sActivityDistributedService MakeK8sActivityDistributedService
	makeK8sActivityStandaloneService  MakeK8sActivityStandaloneService
	makeK8sActivityPreactivityService MakeK8sActivityPreactivityService
}

// New creates a new MakeK8sJobService.
func NewMakeK8sJobService() MakeK8sJobService {
	return MakeK8sJobService{
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,

		makeK8sActivityService: newMakeK8sActivityService(),

		makeK8sActivityDistributedService: newMakeK8sActivityDistributedService(),
		makeK8sActivityStandaloneService:  newMakeK8sActivityStandaloneService(),
		makeK8sActivityPreactivityService: newMakeK8sActivityPreactivityService(),

		mode: MODE_STANDALONE,
	}
}

func (m *MakeK8sJobService) GetNamespace() string {
	return m.namespace
}

func (m *MakeK8sJobService) GetMode() string {
	return m.mode
}

func (m *MakeK8sJobService) GetWorkflow() workflow_entity.Workflow {
	return m.workflow
}

// UsePreactivityMode sets the mode of the k8s job to preactivity.
func (m *MakeK8sJobService) UsePreactivityMode() *MakeK8sJobService {
	m.mode = MODE_PREACTIVITY
	return m
}

// UseDistributedMode sets the mode of the k8s job to distributed.
func (m *MakeK8sJobService) UseDistributedMode() *MakeK8sJobService {
	m.mode = MODE_DISTRIBUTED
	return m
}

// UseStandaloneMode sets the mode of the k8s job to standalone.
func (m *MakeK8sJobService) UseStandaloneMode() *MakeK8sJobService {
	m.mode = MODE_STANDALONE
	return m
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

// GetDependencies returns the dependencies of the activity.
func (m *MakeK8sJobService) GetDependencies() []workflow_activity_entity.WorkflowActivities {
	return m.dependencies
}

// GetIdWorkflow returns the id of the workflow that will be used to make the k8s job.
func (m *MakeK8sJobService) GetIdWorkflow() int {
	return m.idWorkflow
}

// GetIdWorkflowActivity returns the id of the activity that will be used to make the k8s job.
func (m *MakeK8sJobService) GetIdWorkflowActivity() int {
	return m.idWorkflowActivity
}

func (m *MakeK8sJobService) MakeK8sJob() (k8s_job_entity.K8sJob, error) {

	m.makeK8sActivityService.
		SetWorkflow(m.workflow).
		SetIdWorkflowActivity(m.idWorkflowActivity)

	mapMode := map[string]func(service MakeK8sJobService) (k8s_job_entity.K8sJob, error){
		MODE_STANDALONE:  m.makeK8sActivityStandaloneService.Handle,
		MODE_DISTRIBUTED: m.makeK8sActivityDistributedService.Handle,
		MODE_PREACTIVITY: m.makeK8sActivityPreactivityService.Handle,
	}

	return mapMode[m.mode](*m)
}
