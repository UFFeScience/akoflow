package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type WorkerRunActivityStandaloneService struct {
	namespace string

	createNamespaceService CreateNamespaceService
	createPvcService       CreatePVCService
	runPreActivityService  RunPreactivityService

	Workflow workflow_entity.Workflow

	WorkflowActivity workflow_activity_entity.WorkflowActivities
	applyJobService  ApplyJobService
}

func (r *WorkerRunActivityStandaloneService) SetWorkflow(workflow workflow_entity.Workflow) IWorkerRunActivityService {
	r.Workflow = workflow
	return r
}

func (r *WorkerRunActivityStandaloneService) SetWorkflowActivity(workflowActivity workflow_activity_entity.WorkflowActivities) IWorkerRunActivityService {
	r.WorkflowActivity = workflowActivity
	return r
}

func (r *WorkerRunActivityStandaloneService) GetWorkflow() workflow_entity.Workflow {
	return r.Workflow

}

func (r *WorkerRunActivityStandaloneService) GetWorkflowActivity() workflow_activity_entity.WorkflowActivities {
	return r.WorkflowActivity
}

func NewWorkerRunActivityStandaloneService() *WorkerRunActivityStandaloneService {
	return &WorkerRunActivityStandaloneService{
		namespace:              config.App().DefaultNamespace,
		createNamespaceService: NewCreateNamespaceService(),
		createPvcService:       NewCreatePVCService(),
		runPreActivityService:  NewRunPreactivityService(),
		applyJobService:        NewApplyJobService(),
	}
}

func (r *WorkerRunActivityStandaloneService) ApplyJob(activityID int) bool {
	r.applyJobService.ApplyJobStandalone(activityID)
	return true
}

func (r *WorkerRunActivityStandaloneService) HandleResourceToRunJob(activityID int) bool {
	namespace, errNamespace := r.createNamespaceService.GetOrCreateNamespace(r.GetWorkflow(), r.GetWorkflowActivity(), r.namespace)

	pvc, errPvc := r.createPvcService.GetOrCreatePersistentVolumeClainByActivity(r.GetWorkflow(), r.GetWorkflowActivity(), namespace)
	preactivity, _ := r.runPreActivityService.Run(r.GetWorkflowActivity().Id)

	if errNamespace != nil || errPvc != nil {
		println("Error creating namespace or pvc")
		return false
	}

	if !preactivity {
		println("Preactivity not finished")
		return false
	}

	println("Namespace created: ", namespace)
	println("PVC created: ", pvc)

	return true

}
