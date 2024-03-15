package storages_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

type StorageRepository struct {
	tableName string
}

var TableName = "storages"
var Columns = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, namespace TEXT, status INTEGER, storage_mount_path TEXT, storage_class TEXT, storage_size TEXT,  created_at DATETIME DEFAULT CURRENT_TIMESTAMP)"

func New() *StorageRepository {

	database := connector.Database{}
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

type ParamsStorageCreate struct {
	WorkflowId       int
	Namespace        string
	Status           int
	StorageMountPath string
	StorageClass     string
	StorageSize      string
}

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

func (s *StorageRepository) Create(params ParamsStorageCreate) error {

	database := connector.Database{}
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

func (s *StorageRepository) Find(id int) (StorageDatabase, error) {
	database := connector.Database{}
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
