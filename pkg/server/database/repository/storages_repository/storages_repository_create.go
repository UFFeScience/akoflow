package storages_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type ParamsStorageCreate struct {
	WorkflowId            int
	Namespace             string
	Status                int
	StorageMountPath      string
	StorageClass          string
	StorageSize           string
	MapActivitiesKeepDisk map[int]bool
}

func (s *StorageRepository) Create(params ParamsStorageCreate) error {

	database := repository.Database{}
	c := database.Connect()

	for activityId, keepDisk := range params.MapActivitiesKeepDisk {
		_, err := c.Exec(
			"INSERT INTO "+s.tableName+" (workflow_id, activity_id, namespace, status, storage_mount_path, storage_class, storage_size, initial_file_list, end_file_list, initial_disk_spec, end_disk_spec, keep_storage_after_finish) VALUES (?, ? , ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			params.WorkflowId,
			activityId,
			params.Namespace,
			params.Status,
			params.StorageMountPath,
			params.StorageClass,
			params.StorageSize,
			"{}", // initial_file_list
			"{}", // end_file_list
			"{}", // initial_disk_spec
			"{}", // end_disk_spec
			keepDisk,
		)

		if err != nil {
			return err
		}
	}
	err := c.Close()

	if err != nil {
		return err
	}

	return nil
}
