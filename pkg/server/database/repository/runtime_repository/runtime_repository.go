package runtime_repository

import (
	"encoding/json"

	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type RuntimeRepository struct {
	tableName string
}

type IRuntimeRepository interface {
	CreateOrUpdate(name string, status int, metadata map[string]string)
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
