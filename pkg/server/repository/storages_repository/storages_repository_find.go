package storages_repository

import "github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"

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
