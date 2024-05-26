package storages_repository

import "github.com/ovvesley/akoflow/pkg/server/repository"

func (s *StorageRepository) Find(id int) (StorageDatabase, error) {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+s.tableName+" WHERE ID = ?", id)
	if err != nil {
		return StorageDatabase{}, err
	}

	var storageDatabase StorageDatabase
	for rows.Next() {
		err = rows.Scan(&storageDatabase.Id, &storageDatabase.WorkflowId, &storageDatabase.Namespace, &storageDatabase.Status, &storageDatabase.StorageMountPath, &storageDatabase.StorageClass, &storageDatabase.StorageSize)
		if err != nil {
			return StorageDatabase{}, err
		}
	}

	err = c.Close()
	if err != nil {
		return StorageDatabase{}, err
	}

	return storageDatabase, nil
}

func (s *StorageRepository) GetCreatedStorages(namespace string) []StorageDatabase {
	database := repository.Database{}
	c := database.Connect()

	rows, err := c.Query("SELECT * FROM "+s.tableName+" WHERE namespace = ? AND status = ?", namespace, StatusCreated)
	if err != nil {
		return nil
	}

	var storages []StorageDatabase

	for rows.Next() {
		result := StorageDatabase{}
		err = rows.Scan(
			&result.Id,
			&result.WorkflowId,
			&result.ActivityId,
			&result.PvcName,
			&result.Namespace,
			&result.Status,
			&result.StorageMountPath,
			&result.StorageClass,
			&result.StorageSize,
			&result.InitialFileList,
			&result.EndFileList,
			&result.InitialDiskSpec,
			&result.EndDiskSpec,
			&result.KeepStorageAfterFinish,
			&result.Detached,
			&result.CreatedAt)
		if err != nil {
			return nil
		}

		storages = append(storages, result)
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return storages
}
