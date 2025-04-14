package metrics_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/model"
	"github.com/ovvesley/akoflow/pkg/server/repository"
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
	db := repository.GetInstance()

	err := db.CreateOrVerifyTable(model.Metrics{})
	if err != nil {
		println("Error creating table:", err.Error())
		return nil
	}

	return &MetricsRepository{
		tableName: model.Metrics{}.TableName(),
	}
}

type IMetricsRepository interface {
	Create(params ParamsMetricsCreate) error
}
