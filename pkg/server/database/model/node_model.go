package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type Node struct {
	Name         string  `db:"name" sql:"TEXT NOT NULL UNIQUE"`
	Runtime      string  `db:"runtime" sql:"TEXT NOT NULL"`
	Status       int     `db:"status" sql:"INTEGER NOT NULL"`
	CPUUsage     float64 `db:"cpu_usage"`
	CPUMax       float64 `db:"cpu_max"`
	MemoryUsage  float64 `db:"memory_usage"`
	MemoryLimit  float64 `db:"memory_limit"`
	NetworkLimit float64 `db:"network_limit"`
	NetworkUsage float64 `db:"network_usage"`
	CreatedAt    string  `db:"created_at"`
	UpdatedAt    string  `db:"updated_at"`
}

func (Node) TableName() string {
	return "nodes"
}

func (n Node) GetColumns() []string {
	return database.GenericGetColumns(n)
}

func (n Node) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(n)
}

func (n Node) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(n)
}

func (n Node) GetColumnType(column string) string {
	return database.GenericGetColumnType(n, column)
}
