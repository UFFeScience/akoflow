package storages_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (s *StorageRepository) Find(id int) (StorageDatabase, error) {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, workflow_id, activity_id, pvc_name, namespace, status, storage_mount_path, storage_class, storage_size, initial_file_list, end_file_list, initial_disk_spec, end_disk_spec, keep_storage_after_finish, detached, created_at FROM %s WHERE id = %d",
		s.tableName,
		id,
	)

	resp, err := db.Query(query)
	if err != nil {
		return StorageDatabase{}, err
	}

	results := resp["results"].([]interface{})
	if len(results) == 0 {
		return StorageDatabase{}, fmt.Errorf("no result from rqlite")
	}

	resultMap := results[0].(map[string]interface{})
	columns := resultMap["columns"].([]interface{})
	rows := resultMap["rows"].([]interface{})

	if len(rows) == 0 {
		return StorageDatabase{}, fmt.Errorf("storage not found")
	}

	row := rows[0].([]interface{})
	data := map[string]interface{}{}
	for i, col := range columns {
		data[col.(string)] = row[i]
	}

	storage := parseStorageRow(data)
	return storage, nil
}

func (s *StorageRepository) GetCreatedStorages(namespace string) []StorageDatabase {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"SELECT id, workflow_id, activity_id, pvc_name, namespace, status, storage_mount_path, storage_class, storage_size, initial_file_list, end_file_list, initial_disk_spec, end_disk_spec, keep_storage_after_finish, detached, created_at FROM %s WHERE namespace = '%s' AND status = %d",
		s.tableName,
		namespace,
		StatusCreated,
	)

	resp, err := db.Query(query)
	if err != nil {
		return nil
	}

	results := resp["results"].([]interface{})
	if len(results) == 0 {
		return nil
	}

	resultMap := results[0].(map[string]interface{})
	columns := resultMap["columns"].([]interface{})
	rows := resultMap["rows"].([]interface{})

	var storages []StorageDatabase
	for _, rowData := range rows {
		row := rowData.([]interface{})
		data := map[string]interface{}{}
		for i, col := range columns {
			data[col.(string)] = row[i]
		}
		storage := parseStorageRow(data)
		storages = append(storages, storage)
	}

	return storages
}

func parseStorageRow(data map[string]interface{}) StorageDatabase {
	return StorageDatabase{
		Id:                     int(data["id"].(float64)),
		WorkflowId:             int(data["workflow_id"].(float64)),
		ActivityId:             int(data["activity_id"].(float64)),
		PvcName:                toNullableString(data["pvc_name"]),
		Namespace:              data["namespace"].(string),
		Status:                 int(data["status"].(float64)),
		StorageMountPath:       data["storage_mount_path"].(string),
		StorageClass:           data["storage_class"].(string),
		StorageSize:            data["storage_size"].(string),
		InitialFileList:        data["initial_file_list"].(string),
		EndFileList:            data["end_file_list"].(string),
		InitialDiskSpec:        data["initial_disk_spec"].(string),
		EndDiskSpec:            data["end_disk_spec"].(string),
		KeepStorageAfterFinish: int(data["keep_storage_after_finish"].(float64)),
		Detached:               toNullableString(data["detached"]),
		CreatedAt:              data["created_at"].(string),
	}
}

func toNullableString(val interface{}) *string {
	if val == nil {
		return nil
	}
	str := val.(string)
	return &str
}
