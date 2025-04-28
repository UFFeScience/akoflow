package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type Storage struct {
	ID                     int     `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	WorkflowID             int     `db:"workflow_id"`
	ActivityID             int     `db:"activity_id"`
	PvcName                *string `db:"pvc_name"`
	Namespace              string  `db:"namespace"`
	Status                 int     `db:"status"`
	StorageMountPath       string  `db:"storage_mount_path"`
	StorageClass           string  `db:"storage_class"`
	StorageSize            string  `db:"storage_size"`
	InitialFileList        string  `db:"initial_file_list"`
	EndFileList            string  `db:"end_file_list"`
	InitialDiskSpec        string  `db:"initial_disk_spec"`
	EndDiskSpec            string  `db:"end_disk_spec"`
	KeepStorageAfterFinish int     `db:"keep_storage_after_finish"`
	Detached               *string `db:"detached"`                                   // DATETIME ou NULL
	CreatedAt              string  `db:"created_at" sql:"DEFAULT CURRENT_TIMESTAMP"` // DATETIME padrão
}

func (Storage) TableName() string {
	return "storages"
}

func (s Storage) GetColumns() []string {
	return database.GenericGetColumns(s)
}

func (s Storage) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(s)
}
