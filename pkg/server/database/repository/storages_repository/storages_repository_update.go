package storages_repository

import (
	"strconv"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/database/repository"
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

func (s *StorageRepository) UpdateInitialFileListDisk(activityId int, fileDisk string) error {

	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE " + s.tableName + " SET initial_file_list = '" + fileDisk + "' WHERE activity_id = " + strconv.Itoa(activityId))

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}

func (s *StorageRepository) UpdateEndFileListDisk(activityId int, fileDisk string) error {

	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE " + s.tableName + " SET end_file_list = '" + fileDisk + "' WHERE activity_id = " + strconv.Itoa(activityId))

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}

func (s *StorageRepository) UpdateInitialDiskSpec(activityId int, fileSpec string) error {

	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE " + s.tableName + " SET initial_disk_spec = '" + fileSpec + "' WHERE activity_id = " + strconv.Itoa(activityId))

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}

func (s *StorageRepository) UpdateEndDiskSpec(activityId int, fileSpec string) error {

	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE " + s.tableName + " SET end_disk_spec = '" + fileSpec + "' WHERE activity_id = " + strconv.Itoa(activityId))
	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}

func (s *StorageRepository) UpdateDetached(activityId int) error {

	database := repository.Database{}
	c := database.Connect()

	now := time.Now()

	_, err := c.Exec("UPDATE " + s.tableName + " SET detached = '" + now.Format("2006-01-02 15:04:05") + "' WHERE activity_id = " + strconv.Itoa(activityId))
	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}
