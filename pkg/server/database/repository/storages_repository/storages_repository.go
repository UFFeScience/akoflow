package storages_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type StorageRepository struct {
	tableName string
}

var TableName = "storages"
var Columns = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, activity_id INTEGER, pvc_name TEXT,  namespace TEXT, status INTEGER, storage_mount_path TEXT, storage_class TEXT, storage_size TEXT, initial_file_list TEXT, end_file_list TEXT, initial_disk_spec TEXT, end_disk_spec TEXT, keep_storage_after_finish INTEGER, detached DATETIME,  created_at DATETIME DEFAULT CURRENT_TIMESTAMP)"

type StorageDatabase struct {
	Id                     int
	WorkflowId             int
	ActivityId             int
	PvcName                *string
	Namespace              string
	Status                 int
	StorageMountPath       string
	StorageClass           string
	StorageSize            string
	InitialFileList        string
	EndFileList            string
	InitialDiskSpec        string
	EndDiskSpec            string
	KeepStorageAfterFinish int
	Detached               *string
	CreatedAt              string
}

var StatusNotCreated = 1
var StatusCreated = 2
var StatusCompleted = 3

func New() IStorageRepository {

	database := repository.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model.Storage{})
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &StorageRepository{
		tableName: TableName,
	}
}

type IStorageRepository interface {
	Create(params ParamsStorageCreate) error
	Update(params ParamsStorageUpdate) error
	Find(id int) (StorageDatabase, error)
	GetCreatedStorages(namespace string) []StorageDatabase
	UpdateInitialFileListDisk(activityId int, fileDisk string) error
	UpdateEndFileListDisk(activityId int, fileDisk string) error
	UpdateInitialDiskSpec(activityId int, fileSpec string) error
	UpdateEndDiskSpec(activityId int, fileSpec string) error
	UpdateDetached(activityId int) error
}
