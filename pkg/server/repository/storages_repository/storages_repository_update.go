package storages_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/repository"
	"strconv"
)

type ParamsStorageUpdate struct {
	Status     int
	PvcName    string
	ActivityId int
}

func (s *StorageRepository) Update(params ParamsStorageUpdate) error {

	database := repository.Database{}
	c := database.Connect()

	if !(params.Status > 0 && params.PvcName != "" && params.ActivityId > 0) {
		return nil
	}

	_, err := c.Exec("UPDATE " + s.tableName + " SET status = " + strconv.Itoa(params.Status) + ", pvc_name = '" + params.PvcName + "' WHERE activity_id = " + strconv.Itoa(params.ActivityId))

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil

}
