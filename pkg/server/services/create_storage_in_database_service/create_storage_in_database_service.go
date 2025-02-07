package create_storage_in_database_service

import (
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_map_activities_keep_disk_service"
)

type CreateStorageInDatabaseService struct {
	namespace                       string
	workflowRepository              workflow_repository.IWorkflowRepository
	activityRepository              activity_repository.IActivityRepository
	storageRepository               storages_repository.IStorageRepository
	getMapActivitiesKeepDiskService get_map_activities_keep_disk_service.GetMapActivitiesKeepDiskService
}

func New() CreateStorageInDatabaseService {
	return CreateStorageInDatabaseService{
		namespace:                       "akoflow",
		workflowRepository:              workflow_repository.New(),
		activityRepository:              activity_repository.New(),
		storageRepository:               storages_repository.New(),
		getMapActivitiesKeepDiskService: get_map_activities_keep_disk_service.New(),
	}
}

func (c *CreateStorageInDatabaseService) CreateByWorkflow(wfId int) error {

	workflow, err := c.workflowRepository.Find(wfId)
	if err != nil {
		return err
	}

	mapActivitiesKeepDisk, _ := c.getMapActivitiesKeepDiskService.GetMapActivitiesKeepDisk(workflow.Id)

	println("mapActivitiesKeepDisk", len(mapActivitiesKeepDisk))

	err = c.storageRepository.Create(storages_repository.ParamsStorageCreate{
		WorkflowId:            wfId,
		Namespace:             c.namespace,
		Status:                storages_repository.StatusCreated,
		MapActivitiesKeepDisk: mapActivitiesKeepDisk,
		StorageMountPath:      workflow.Spec.MountPath,
		StorageClass:          workflow.Spec.StoragePolicy.StorageClassName,
		StorageSize:           workflow.Spec.StoragePolicy.StorageSize,
	})

	return nil

}
