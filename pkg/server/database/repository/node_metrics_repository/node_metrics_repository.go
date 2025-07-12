package node_metrics_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type NodeMetricsRepository struct {
	tableName string
}

const STATUS_READY = 1
const STATUS_NOT_READY = 0

type INodeMetricsRepository interface {
	CreateOrUpdate(name string, status int, metadata map[string]string)
}

func New() INodeMetricsRepository {

	database := repository.Database{}
	model := model.NodeMetrics{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model)
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &NodeMetricsRepository{
		tableName: model.TableName(),
	}
}

func (nmr *NodeMetricsRepository) CreateOrUpdate(name string, status int, metadata map[string]string) {
	// Implementation for creating or updating a node metrics entry
	// This would typically involve inserting or updating the record in the database
	// using the provided name, status, and metadata.

}
