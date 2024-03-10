package logs_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
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

func New() *LogsRepository {

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

	return &LogsRepository{
		tableName: TableName,
	}
}

type ParamsLogsCreate struct {
	LogsDatabase LogsDatabase
}

func (l *LogsRepository) CreateOrUpdate(params ParamsLogsCreate) error {

	database := connector.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"INSERT INTO "+l.tableName+" (activity_id, logs) VALUES (?, ?)",
		params.LogsDatabase.ActivityId, params.LogsDatabase.Logs)

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil

}
