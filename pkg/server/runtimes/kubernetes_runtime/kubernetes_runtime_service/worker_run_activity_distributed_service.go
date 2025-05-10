package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type WorkerRunActivityDistributedService struct {
	namespace string

	createPvcService CreatePVCService
	createNfsService CreateNfsService

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
		createPvcService: NewCreatePVCService(),
		createNfsService: NewCreateNfsService(),
		applyJobService:  NewApplyJobService(),
	}
}

func (r *WorkerRunActivityDistributedService) ApplyJob(activityID int) bool {
	r.applyJobService.ApplyJobDistributed(activityID)
	return true
}

func (r *WorkerRunActivityDistributedService) HandleResourceToRunJob(activityID int) bool {
	wf := r.GetWorkflow()
	wfa := r.GetWorkflowActivity()

	r.createNfsService.
		SetNamespace(wf.Spec.Namespace).
		SetActivity(wfa).
		SetWorkflow(wf)

	if r.createNfsService.NfsServerIsCreated() {
		return true
	}

	return r.createNfsService.Create()

}
