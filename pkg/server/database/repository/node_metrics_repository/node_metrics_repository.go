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
	Create(params ParamsNodeMetricsCreate) error
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

func (nmr *NodeMetricsRepository) Create(params ParamsNodeMetricsCreate) error {
	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"INSERT INTO "+nmr.tableName+" (node_id, cpu_usage, cpu_memory, memory_usage, memory_limit, network_usage, network_limit, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		params.NodeMetricsDatabase.NodeID,
		params.NodeMetricsDatabase.CpuUsage,
		params.NodeMetricsDatabase.CpuMemory,
		params.NodeMetricsDatabase.MemoryUsage,
		params.NodeMetricsDatabase.MemoryLimit,
		params.NodeMetricsDatabase.NetworkUsage,
		params.NodeMetricsDatabase.NetworkLimit,
		params.NodeMetricsDatabase.Timestamp)
	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}

type ParamsNodeMetricsCreate struct {
	NodeMetricsDatabase NodeMetricsDatabase
}
type NodeMetricsDatabase struct {
	NodeID       string
	CpuUsage     string
	CpuMemory    string
	MemoryUsage  string
	MemoryLimit  string
	NetworkUsage string
	NetworkLimit string
	Timestamp    string
}
