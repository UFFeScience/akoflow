package metrics_repository

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository"
)

type MetricsRepository struct {
	tableName string
}

var TableName = "metrics"
var Columns = "(ID INTEGER PRIMARY KEY AUTOINCREMENT, activity_id INTEGER, cpu TEXT, memory TEXT, window TEXT, timestamp TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)"

type MetricsDatabase struct {
	ID         int
	ActivityId int
	Cpu        string
	Memory     string
	Window     string
	Timestamp  string
	CreatedAt  string
}

func New() *MetricsRepository {

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

	return &MetricsRepository{
		tableName: TableName,
	}
}

type ParamsMetricsCreate struct {
	MetricsDatabase MetricsDatabase
}

func (m *MetricsRepository) Create(params ParamsMetricsCreate) error {

	database := connector.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"INSERT INTO "+m.tableName+" (activity_id, cpu, memory, window, timestamp) VALUES (?, ?, ?, ?, ?)",
		params.MetricsDatabase.ActivityId, params.MetricsDatabase.Cpu, params.MetricsDatabase.Memory, params.MetricsDatabase.Window, params.MetricsDatabase.Timestamp)

	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}
