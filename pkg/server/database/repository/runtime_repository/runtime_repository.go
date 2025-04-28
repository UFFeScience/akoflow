package runtime_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type RuntimeRepository struct {
	tableName string
}

type IRuntimeRepository interface {
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
