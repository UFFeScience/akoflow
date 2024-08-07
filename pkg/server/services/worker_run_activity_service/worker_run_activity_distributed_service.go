package worker_run_activity_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/services/apply_job_service"
	"github.com/ovvesley/akoflow/pkg/server/services/create_nfs_service"
	"github.com/ovvesley/akoflow/pkg/server/services/create_pvc_service"
)

type WorkerRunActivityDistributedService struct {
	namespace string

	createPvcService create_pvc_service.CreatePVCService
	createNfsService create_nfs_service.CreateNfsService

	Workflow         workflow_entity.Workflow
	WorkflowActivity workflow_activity_entity.WorkflowActivities

	applyJobService ApplyJobService
}

func (r *WorkerRunActivityDistributedService) SetWorkflow(workflow workflow_entity.Workflow) IWorkerRunActivityService {
	r.Workflow = workflow
	return r
}

func (r *WorkerRunActivityDistributedService) SetWorkflowActivity(workflowActivity workflow_activity_entity.WorkflowActivities) IWorkerRunActivityService {
	r.WorkflowActivity = workflowActivity
	return r
}

func (r *WorkerRunActivityDistributedService) GetWorkflow() workflow_entity.Workflow {
	return r.Workflow
}

func (r *WorkerRunActivityDistributedService) GetWorkflowActivity() workflow_activity_entity.WorkflowActivities {
	return r.WorkflowActivity
}

func NewWorkerRunActivityDistributedService() *WorkerRunActivityDistributedService {
	return &WorkerRunActivityDistributedService{
		namespace:        config.App().DefaultNamespace,
		createPvcService: create_pvc_service.New(),
		createNfsService: create_nfs_service.New(),
		applyJobService:  apply_job_service.New(),
	}
}

func (r *WorkerRunActivityDistributedService) ApplyJob(activityID int) bool {
	r.applyJobService.ApplyDistributedJob(activityID)
	return true
}

func (r *WorkerRunActivityDistributedService) HandleResourceToRunJob(activityID int) bool {

	if !r.createNfsService.SetWorkflowId(r.GetWorkflow().Id).Create() {
		println("Error creating nfs")
		return false
	}

	r.createPvcService.GetOrCreatePersistentVolumeClainByActivity(r.GetWorkflow(), r.GetWorkflowActivity(), r.namespace)

	return true
}
