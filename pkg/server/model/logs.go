package model

import "github.com/ovvesley/akoflow/pkg/server/utils/utils_model"

type Logs struct {
	ID         int    `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	ActivityID int    `db:"activity_id"`
	Logs       string `db:"logs"`
	CreatedAt  string `db:"created_at" sql:"DEFAULT CURRENT_TIMESTAMP"`
}

func (Logs) TableName() string {
	return "logs"
}

func (l Logs) GetColumns() []string {
	return utils_model.GenericGetColumns(l)
}

func (l Logs) GetPrimaryKey() string {
	return utils_model.GenericGetPrimaryKey(l)
}
