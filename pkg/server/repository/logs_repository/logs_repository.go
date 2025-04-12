package logs_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/model"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type LogsRepository struct {
	tableName string
}

var TableName = "logs"
var Columns = "(ID INTEGER PRIMARY KEY AUTOINCREMENT, activity_id INTEGER, logs TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)"

type LogsDatabase struct {
	ID         int
	ActivityId int
	Logs       string
	CreatedAt  string
}

func New() ILogsRepository {
	db := repository.GetInstance()

	err := db.CreateOrVerifyTable(model.Logs{})
	if err != nil {
		println("Error creating table:", err.Error())
		return nil
	}

	return &LogsRepository{
		tableName: model.Logs{}.TableName(),
	}
}

type ILogsRepository interface {
	Create(params ParamsLogsCreate) error
}
