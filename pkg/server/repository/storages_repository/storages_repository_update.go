package storages_repository

import "github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"

type ParamsStorageUpdate struct {
	Status int
}

func (s *StorageRepository) Update(params ParamsStorageUpdate) error {

	database := connector.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"UPDATE "+s.tableName+" SET status = ?",
		params.Status)

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}
