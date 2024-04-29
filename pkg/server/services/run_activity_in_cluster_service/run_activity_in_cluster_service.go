package run_activity_in_cluster_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/services/apply_job_service"
	"github.com/ovvesley/scik8sflow/pkg/server/services/create_namespace_service"
	"github.com/ovvesley/scik8sflow/pkg/server/services/create_pvc_service"
	"github.com/ovvesley/scik8sflow/pkg/server/services/run_preactivity_service"
)

type RunActivityInClusterService struct {
	namespace              string
	workflowRepository     workflow_repository.IWorkflowRepository
	activityRepository     activity_repository.IActivityRepository
	createPVCService       create_pvc_service.CreatePVCService
	createNamespaceService create_namespace_service.CreateNamespaceService
	applyJobService        apply_job_service.ApplyJobService
	runPreactivityService  run_preactivity_service.RunPreactivityService
}

type ParamsNewRunActivityInClusterService struct {
	Namespace              string
	WorkflowRepository     workflow_repository.IWorkflowRepository
	ActivityRepository     activity_repository.IActivityRepository
	CreatePVCService       create_pvc_service.CreatePVCService
	CreateNamespaceService create_namespace_service.CreateNamespaceService
	ApplyJobService        apply_job_service.ApplyJobService
	RunPreactivityService  run_preactivity_service.RunPreactivityService
}

func New(params ...ParamsNewRunActivityInClusterService) *RunActivityInClusterService {
	if len(params) > 0 {
		return &RunActivityInClusterService{
			namespace:              params[0].Namespace,
			workflowRepository:     params[0].WorkflowRepository,
			activityRepository:     params[0].ActivityRepository,
			createPVCService:       params[0].CreatePVCService,
			createNamespaceService: params[0].CreateNamespaceService,
			applyJobService:        params[0].ApplyJobService,
			runPreactivityService:  params[0].RunPreactivityService,
		}
	}

	return &RunActivityInClusterService{
		namespace:              "scik8sflow",
		workflowRepository:     workflow_repository.New(),
		activityRepository:     activity_repository.New(),
		createPVCService:       create_pvc_service.New(),
		createNamespaceService: create_namespace_service.New(),
		applyJobService:        apply_job_service.New(),
		runPreactivityService:  run_preactivity_service.New(),
	}
}

func (r *RunActivityInClusterService) Run(activityID int) {
	resourceOk := r.handleResourceToRunJob(activityID)

	if resourceOk {
		r.applyJobService.ApplyJob(activityID)
	}

}

func (r *RunActivityInClusterService) handleResourceToRunJob(id int) bool {
	wfa, err := r.activityRepository.Find(id)
	wf, _ := r.workflowRepository.Find(wfa.WorkflowId)

	if err != nil {
		println("Activity not found")
		return false
	}

	namespace, errNamespace := r.createNamespaceService.GetOrCreateNamespace(r.namespace)

	pvc, errPvc := r.createPVCService.GetOrCreatePersistentVolumeClainByActivity(wf, wfa, namespace)

	preactivity, _ := r.runPreactivityService.Run(wfa.Id)

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
