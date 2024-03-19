package metrics_repository

import (
	"github.com/ovvesley/scik8sflow/pkg/server/repository"
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

	return &MetricsRepository{
		tableName: TableName,
	}
}

type IMetricsRepository interface {
	Create(params ParamsMetricsCreate) error
}
