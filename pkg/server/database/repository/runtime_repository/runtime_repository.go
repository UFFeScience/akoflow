package runtime_repository

import (
	"encoding/json"

	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type RuntimeRepository struct {
	tableName string
}

const STATUS_READY = 1
const STATUS_NOT_READY = 0

type IRuntimeRepository interface {
	CreateOrUpdate(name string, status int, metadata map[string]string)

	GetAll() ([]runtime_entity.Runtime, error)
	GetByName(name string) (*runtime_entity.Runtime, error)

	UpdateStatus(runtime *runtime_entity.Runtime, status int) error
}

func New() IRuntimeRepository {

	database := repository.Database{}
	model := model.Runtime{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model)
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &RuntimeRepository{
		tableName: model.TableName(),
	}
}

func (r *RuntimeRepository) CreateOrUpdate(name string, status int, metadata map[string]string) {
	database := repository.Database{}
	c := database.Connect()

	if c == nil {
		return
	}

	metadataString := "{}"

	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return
		}
		metadataString = string(metadataBytes)
	}

	result, err := c.Query("SELECT COUNT(*) FROM "+r.tableName+" WHERE name = ?", name)

	if err != nil {
		return
	}

	var count int

	if result.Next() {
		err = result.Scan(&count)
		if err != nil {
			return
		}
	} else {
		return
	}
	err = result.Close()
	if err != nil {
		return
	}

	// Check if the runtime already exists
	c = database.Connect()
	defer c.Close()

	println("count", count)
	println("metadataString", metadataString)

	if count == 0 {
		_, err = c.Exec("INSERT INTO "+r.tableName+" (name, status, metadata, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))", name, status, metadataString)
	} else {
		_, err = c.Exec("UPDATE "+r.tableName+" SET status = ?, metadata = ?, updated_at = datetime('now') WHERE name = ?", status, metadataString, name)
	}

	if err != nil {
		return
	}

}

func (r *RuntimeRepository) GetAll() ([]runtime_entity.Runtime, error) {
	database := repository.Database{}
	c := database.Connect()

	if c == nil {
		return nil, nil
	}

	defer c.Close()

	rows, err := c.Query("SELECT * FROM " + r.tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runtimes []runtime_entity.Runtime

	for rows.Next() {
		var runtime model.Runtime
		runtimeMetadata := "{}"
		err = rows.Scan(&runtime.Name, &runtime.Status, &runtimeMetadata, &runtime.CreatedAt, &runtime.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Unmarshal the metadata string into a map
		var metadata map[string]string
		err = json.Unmarshal([]byte(runtimeMetadata), &metadata)
		if err != nil {
			return nil, err
		}

		runtimeEntity := runtime_entity.NewRuntime(runtime.Name, runtime.Status, metadata, runtime.CreatedAt, runtime.UpdatedAt)
		runtimes = append(runtimes, *runtimeEntity)
	}

	return runtimes, nil
}

func (r *RuntimeRepository) GetByName(name string) (*runtime_entity.Runtime, error) {
	database := repository.Database{}
	c := database.Connect()

	if c == nil {
		return nil, nil
	}

	defer c.Close()

	var runtime model.Runtime
	runtimeMetadata := "{}"
	err := c.QueryRow("SELECT name, status, metadata, created_at, updated_at FROM "+r.tableName+" WHERE name = ?", name).Scan(&runtime.Name, &runtime.Status, &runtimeMetadata, &runtime.CreatedAt, &runtime.UpdatedAt)
	if err != nil {
		return nil, err
	}
	// Unmarshal the metadata string into a map
	var metadata map[string]string
	err = json.Unmarshal([]byte(runtimeMetadata), &metadata)
	if err != nil {
		return nil, err
	}
	runtimeEntity := runtime_entity.NewRuntime(runtime.Name, runtime.Status, metadata, runtime.CreatedAt, runtime.UpdatedAt)
	return runtimeEntity, nil
}

func (r *RuntimeRepository) UpdateStatus(runtime *runtime_entity.Runtime, status int) error {
	database := repository.Database{}
	c := database.Connect()

	if c == nil {
		return nil
	}

	defer c.Close()

	_, err := c.Exec("UPDATE "+r.tableName+" SET status = ?, updated_at = datetime('now') WHERE name = ?", status, runtime.GetName())
	if err != nil {
		return err
	}

	return nil
}
