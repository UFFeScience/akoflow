package get_map_activities_keep_disk_service

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_activity_dependencies_service"
)

type GetMapActivitiesKeepDiskService struct {
	namespace                     string
	workflowRepository            workflow_repository.IWorkflowRepository
	activityRepository            activity_repository.IActivityRepository
	storageRepository             storages_repository.IStorageRepository
	getActivityDependeciesService get_activity_dependencies_service.GetActivityDependenciesService
}

func New() GetMapActivitiesKeepDiskService {
	return GetMapActivitiesKeepDiskService{
		namespace:                     "akoflow",
		workflowRepository:            workflow_repository.New(),
		activityRepository:            activity_repository.New(),
		storageRepository:             storages_repository.New(),
		getActivityDependeciesService: get_activity_dependencies_service.New(),
	}
}

type MapActivitiesKeepDisk map[int]bool

func (c *GetMapActivitiesKeepDiskService) GetMapActivitiesKeepDisk(wfId int) (MapActivitiesKeepDisk, error) {

	activitiesByWorkflow, err := c.activityRepository.GetActivitiesByWorkflowIds([]int{wfId})
	activitiesDependencies := c.getActivityDependeciesService.GetActivityDependencies(wfId)

	if err != nil {
		return nil, err
	}

	activities := activitiesByWorkflow[wfId]

	mapActivitiesKeepDisk := make(map[int]bool)

	// default keep all disk
	for _, activity := range activities {
		mapActivitiesKeepDisk[activity.Id] = true
	}

	for _, dependencies := range activitiesDependencies {
		for _, dependency := range dependencies {
			if !dependency.KeepDisk {
				mapActivitiesKeepDisk[dependency.Id] = false
			}
		}
	}

	return mapActivitiesKeepDisk, nil
}
