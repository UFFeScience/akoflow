package storages_repository

import "github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"

type ParamsStorageCreate struct {
	WorkflowId       int
	Namespace        string
	Status           int
	StorageMountPath string
	StorageClass     string
	StorageSize      string
}

func (s *StorageRepository) Create(params ParamsStorageCreate) error {

	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"INSERT INTO "+s.tableName+" (workflow_id, namespace, status, storage_mount_path, storage_class, storage_size) VALUES (?, ?, ?, ?, ?, ?)",
		params.WorkflowId, params.Namespace, params.Status, params.StorageMountPath, params.StorageClass, params.StorageSize)

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}
