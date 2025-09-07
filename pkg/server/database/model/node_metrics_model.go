package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type NodeMetrics struct {
	ID           int     `db:"id" sql:"INTEGER PRIMARY KEY AUTOINCREMENT"`
	NodeID       int     `db:"node_id"`
	CpuUsage     float64 `db:"cpu_usage"`
	CpuMemory    float64 `db:"cpu_memory"`
	MemoryUsage  float64 `db:"memory_usage"`
	MemoryLimit  float64 `db:"memory_limit"`
	NetworkUsage float64 `db:"network_usage"`
	NetworkLimit float64 `db:"network_limit"`
	Timestamp    string  `db:"timestamp"`
}

func (NodeMetrics) TableName() string {
	return "nodes_metrics"
}

func (n NodeMetrics) GetColumns() []string {
	return database.GenericGetColumns(n)
}

func (n NodeMetrics) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(n)
}

func (n NodeMetrics) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(n)
}

func (n NodeMetrics) GetColumnType(column string) string {
	return database.GenericGetColumnType(n, column)
}
