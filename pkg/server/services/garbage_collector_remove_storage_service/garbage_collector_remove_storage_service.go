package garbage_collector_remove_storage_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_activity_dependencies_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_pending_storage_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_workflow_by_status_service"
	"strconv"
)

type GarbageCollectorRemoveStorageService struct {
	namespace          string
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
	storageRepository  storages_repository.IStorageRepository

	getActivityDependeciesService get_activity_dependencies_service.GetActivityDependenciesService
	getPendingWorkflowService     get_pending_workflow_service.GetPendingWorkflowService
	getWorkflowByStatusService    get_workflow_by_status_service.GetWorkflowByStatusService
	getPendingStorageService      get_pending_storage_service.GetPendingStorageService
	connector                     connector.IConnector
}

func New() GarbageCollectorRemoveStorageService {
	return GarbageCollectorRemoveStorageService{
		namespace:          "akoflow",
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
		storageRepository:  config.App().Repository.StoragesRepository,

		connector: config.App().Connector.K8sConnector,

		getActivityDependeciesService: get_activity_dependencies_service.New(),
		getPendingWorkflowService:     get_pending_workflow_service.New(),
		getWorkflowByStatusService:    get_workflow_by_status_service.New(),
		getPendingStorageService:      get_pending_storage_service.New(),
	}
}

type MapActivitiesKeepDisk map[int]bool

func (c *GarbageCollectorRemoveStorageService) RemoveStorages() {
	wfaFinished, _ := c.getPendingStorageService.GetPendingStorages()

	wfaPreactivities, _ := c.activityRepository.GetPreactivitiesCompleted()

	for _, activity := range wfaFinished {
		if activity.KeepDisk {
			c.handleKeepDisk(activity)
		} else {
			c.removeResource(activity)
		}
	}

	for _, preactivity := range wfaPreactivities {

		podJob, _ := c.connector.Pod().GetPodByJob(c.namespace, "preactivity-"+strconv.Itoa(preactivity.ActivityId))
		podNameJob, _ := podJob.GetPodName()

		_ = c.connector.Pod().DeletePod(c.namespace, podNameJob)

		preactivity.Status = activity_repository.StatusCompleted
		_ = c.activityRepository.UpdatePreActivity(preactivity.ActivityId, preactivity)

	}

}

func (c *GarbageCollectorRemoveStorageService) removeResource(activity workflow_activity_entity.WorkflowActivities) {
	_ = c.connector.PersistentVolumeClain().DeletePersistentVolumeClaim(activity.GetVolumeName(), c.namespace)

	podJob, _ := c.connector.Pod().GetPodByJob(c.namespace, activity.GetNameJob())
	podNameJob, _ := podJob.GetPodName()

	_ = c.connector.Pod().DeletePod(c.namespace, podNameJob)

	_ = c.storageRepository.Update(storages_repository.ParamsStorageUpdate{
		PvcName:    activity.GetVolumeName(),
		Status:     storages_repository.StatusCompleted,
		ActivityId: activity.Id,
	})

	_ = c.storageRepository.UpdateDetached(activity.Id)
}

func (c *GarbageCollectorRemoveStorageService) handleKeepDisk(activity workflow_activity_entity.WorkflowActivities) {
	err := c.storageRepository.Update(storages_repository.ParamsStorageUpdate{
		PvcName:    activity.GetVolumeName(),
		Status:     storages_repository.StatusCompleted,
		ActivityId: activity.Id,
	})
	if err != nil {
		println("Error updating storage")
		return
	}
}
