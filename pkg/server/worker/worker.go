package worker

import (
	"github.com/ovvesley/akoflow/pkg/server/channel"
	"github.com/ovvesley/akoflow/pkg/server/services/run_activity_in_cluster_service"
)

func StartWorker() {

	for {
		managerChannel := channel.GetInstance()
		result := <-managerChannel.WorfklowChannel
		handleWorker(result)
		println("Worker is Listening...")
	}
}

func handleWorker(result channel.DataChannel) {

	//// Root Dependencies
	//// namespace: "akoflow",
	//// workflowRepository: workflow_repository.New(),
	//// activityRepository: activity_repository.New(),
	//// createPVCService: create_pvc_service.New(
	////     connector: connector.New(),
	////     storageRepository: storages_repository.New(),
	//// ),
	//// createNamespaceService: create_namespace_service.New(
	////     ParamsNewCreateNamespaceService{
	////         Connector: connector.New(),
	////     }
	//// ),
	//// applyJobService: apply_job_service.New(
	////     activityRepository: activity_repository.New(),
	////     workflowRepository: workflow_repository.New(),
	////     connector: connector.New(),
	////     namespace: "akoflow",
	////     getActivityDependenciesService: get_activity_dependencies_service.New(),
	////     makeK8sJobService: make_k8s_job_service.New(),
	//// ),
	//// runPreactivityService: run_preactivity_service.New(
	////     namespace: "akoflow",
	////     workflowRepository: workflow_repository.New(),
	////     activityRepository: activity_repository.New(),
	////     makeK8sJobService: make_k8s_job_service.New(),
	////     getActivityDependenciesService: get_activity_dependencies_service.New(),
	////     connector: connector.New(),
	//// ),
	//// createNfsService: create_nfs_service.New()
	runActivityInClusterService := run_activity_in_cluster_service.New()

	runActivityInClusterService.Run(result.Id)
}
