package get_pending_storage_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_activity_dependencies_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_workflow_by_status_service"
)

type GetPendingStorageService struct {
	namespace                  string
	workflowRepository         workflow_repository.IWorkflowRepository
	activityRepository         activity_repository.IActivityRepository
	storageRepository          storages_repository.IStorageRepository
	getWorkflowByStatusService get_workflow_by_status_service.GetWorkflowByStatusService
	getActivityDependencies    get_activity_dependencies_service.GetActivityDependenciesService

	connector connector.IConnector
}

func New() GetPendingStorageService {
	return GetPendingStorageService{
		namespace:          "akoflow",
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
		storageRepository:  config.App().Repository.StoragesRepository,

		getWorkflowByStatusService: get_workflow_by_status_service.New(),
		getActivityDependencies:    get_activity_dependencies_service.New(),

		connector: connector.New(),
	}
}

func (g *GetPendingStorageService) GetPendingStorages() ([]workflow_activity_entity.WorkflowActivities, error) {
	storages := g.storageRepository.GetCreatedStorages(g.namespace)

	mapWorkflowByStorage := make(map[int][]storages_repository.StorageDatabase)
	mapActivityByStorage := make(map[int]storages_repository.StorageDatabase)
	workflowsIds := make([]int, 0)

	for _, storage := range storages {
		mapWorkflowByStorage[storage.WorkflowId] = append(mapWorkflowByStorage[storage.WorkflowId], storage)
		mapActivityByStorage[storage.ActivityId] = storage
	}

	for key := range mapWorkflowByStorage {
		workflowsIds = append(workflowsIds, key)
	}

	wfActivities, _ := g.activityRepository.GetActivitiesByWorkflowIds(workflowsIds)

	allActivities := make([]workflow_activity_entity.WorkflowActivities, 0)

	for wfaId, activities := range wfActivities {
		allActivities = append(allActivities, g.handleWorkflowActivities(wfaId, activities, mapWorkflowByStorage[wfaId])...)
	}

	allActivitiesFiltered := make([]workflow_activity_entity.WorkflowActivities, 0)
	for _, activity := range allActivities {
		storage := mapActivityByStorage[activity.Id]

		if storage.Id == 0 {
			continue
		}

		if storage.KeepStorageAfterFinish == 1 {
			activity.KeepDisk = true
		}

		if storage.Status == storages_repository.StatusCompleted {
			continue
		}

		allActivitiesFiltered = append(allActivitiesFiltered, activity)

	}

	return allActivitiesFiltered, nil
}

func (g *GetPendingStorageService) handleWorkflowActivities(wfId int, activities []workflow_activity_entity.WorkflowActivities, storages []storages_repository.StorageDatabase) []workflow_activity_entity.WorkflowActivities {

	wfaFinisheds := g.getWorkflowByStatusService.GetActivitiesByStatuses(activities, activity_repository.StatusFinished)
	wfaRunning := g.getWorkflowByStatusService.GetActivitiesByStatuses(activities, activity_repository.StatusRunning)

	wfaStarted := append(wfaFinisheds, wfaRunning...)

	mapWfaToBeDeleted := make(map[int]workflow_activity_entity.WorkflowActivities)
	for _, activity := range wfaStarted {
		mapWfaToBeDeleted[activity.Id] = activity
	}

	allDependencies := g.getActivityDependencies.GetActivityDependenciesByWorkflow(wfId)

	activitiesToDelete := make([]workflow_activity_entity.WorkflowActivities, 0)

	workflowThatNeedByActivity := make(map[int][]int)
	for activityId, dependencies := range allDependencies {
		for _, dependency := range dependencies {
			workflowThatNeedByActivity[dependency.Id] = append(workflowThatNeedByActivity[dependency.Id], activityId)
		}
	}

	for _, wfaFinished := range wfaFinisheds {
		activitiesShouldStarted := workflowThatNeedByActivity[wfaFinished.Id]
		activitiesStarted := make([]workflow_activity_entity.WorkflowActivities, 0)
		for _, activityShouldStarted := range activitiesShouldStarted {
			if _, ok := mapWfaToBeDeleted[activityShouldStarted]; ok {
				activitiesStarted = append(activitiesStarted, mapWfaToBeDeleted[activityShouldStarted])
			}
		}

		if len(activitiesStarted) == len(activitiesShouldStarted) {
			activitiesToDelete = append(activitiesToDelete, wfaFinished)
		}

	}

	return activitiesToDelete

}
