package storages_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/repository"
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
	db := repository.GetInstance()

	for activityId, keepDisk := range params.MapActivitiesKeepDisk {
		query := fmt.Sprintf(`
			INSERT INTO %s (
				workflow_id, activity_id, namespace, status,
				storage_mount_path, storage_class, storage_size,
				initial_file_list, end_file_list,
				initial_disk_spec, end_disk_spec, keep_storage_after_finish
			) VALUES (
				%d, %d, '%s', %d,
				'%s', '%s', '%s',
				'{}', '{}',
				'{}', '{}', %t
			)`,
			s.tableName,
			params.WorkflowId,
			activityId,
			params.Namespace,
			params.Status,
			params.StorageMountPath,
			params.StorageClass,
			params.StorageSize,
			keepDisk,
		)

		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}

	return nil
}
