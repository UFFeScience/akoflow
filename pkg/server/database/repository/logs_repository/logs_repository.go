package logs_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
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

	return &LogsRepository{
		tableName: TableName,
	}
}

type ILogsRepository interface {
	Create(params ParamsLogsCreate) error
}
