package storages_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type StorageRepository struct {
	tableName string
}

var TableName = "storages"
var Columns = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, namespace TEXT, status INTEGER, storage_mount_path TEXT, storage_class TEXT, storage_size TEXT,  created_at DATETIME DEFAULT CURRENT_TIMESTAMP)"

type StorageDatabase struct {
	Id               int
	WorkflowId       int
	Namespace        string
	Status           int
	StorageMountPath string
	StorageClass     string
	StorageSize      string
	CreatedAt        string
}

var StatusCreated = 0
var StatusCreating = 1
var StatusCreatedError = 2
var StatusCreatedSuccess = 3

func New() IStorageRepository {

	database := repository.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, TableName, Columns)
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
}
