package model

import (
	"github.com/ovvesley/akoflow/pkg/server/database"
)

type Metrics struct {
	ID         int    `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	ActivityID int    `db:"activity_id"`
	Cpu        string `db:"cpu"`
	Memory     string `db:"memory"`
	Window     string `db:"window"`
	Timestamp  string `db:"timestamp"`
	CreatedAt  string `db:"created_at" sql:"DEFAULT CURRENT_TIMESTAMP"`
}

func (Metrics) TableName() string {
	return "metrics"
}

func (m Metrics) GetColumns() []string {
	return database.GenericGetColumns(m)
}

func (m Metrics) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(m)
}
