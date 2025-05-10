package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type Logs struct {
	ID         int    `db:"id" sql:"INTEGER PRIMARY KEY AUTOINCREMENT"`
	ActivityID int    `db:"activity_id"`
	Logs       string `db:"logs"`
	CreatedAt  string `db:"created_at" sql:"DEFAULT CURRENT_TIMESTAMP"`
}

func (Logs) TableName() string {
	return "logs"
}

func (l Logs) GetColumns() []string {
	return database.GenericGetColumns(l)
}

func (l Logs) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(l)
}

func (l Logs) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(l)
}

func (l Logs) GetColumnType(column string) string {
	return database.GenericGetColumnType(l, column)
}
