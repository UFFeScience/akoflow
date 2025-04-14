package storages_repository

import (
	"fmt"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type ParamsStorageUpdate struct {
	Status     int
	PvcName    string
	ActivityId int
}

func (s *StorageRepository) Update(params ParamsStorageUpdate) error {
	if !(params.Status > 0 && params.PvcName != "" && params.ActivityId > 0) {
		return nil
	}

	db := repository.GetInstance()

	query := fmt.Sprintf(
		"UPDATE %s SET status = %d, pvc_name = '%s' WHERE activity_id = %d",
		s.tableName,
		params.Status,
		params.PvcName,
		params.ActivityId,
	)

	_, err := db.Exec(query)
	return err
}

func (s *StorageRepository) UpdateInitialFileListDisk(activityId int, fileDisk string) error {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"UPDATE %s SET initial_file_list = '%s' WHERE activity_id = %d",
		s.tableName,
		fileDisk,
		activityId,
	)

	_, err := db.Exec(query)
	return err
}

func (s *StorageRepository) UpdateEndFileListDisk(activityId int, fileDisk string) error {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"UPDATE %s SET end_file_list = '%s' WHERE activity_id = %d",
		s.tableName,
		fileDisk,
		activityId,
	)

	_, err := db.Exec(query)
	return err
}

func (s *StorageRepository) UpdateInitialDiskSpec(activityId int, fileSpec string) error {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"UPDATE %s SET initial_disk_spec = '%s' WHERE activity_id = %d",
		s.tableName,
		fileSpec,
		activityId,
	)

	_, err := db.Exec(query)
	return err
}

func (s *StorageRepository) UpdateEndDiskSpec(activityId int, fileSpec string) error {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"UPDATE %s SET end_disk_spec = '%s' WHERE activity_id = %d",
		s.tableName,
		fileSpec,
		activityId,
	)

	_, err := db.Exec(query)
	return err
}

func (s *StorageRepository) UpdateDetached(activityId int) error {
	db := repository.GetInstance()

	now := time.Now().Format("2006-01-02 15:04:05")

	query := fmt.Sprintf(
		"UPDATE %s SET detached = '%s' WHERE activity_id = %d",
		s.tableName,
		now,
		activityId,
	)

	_, err := db.Exec(query)
	return err
}
