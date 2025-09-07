package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type Runtime struct {
	Name      string `db:"name" sql:"TEXT PRIMARY KEY"`
	Status    int    `db:"status"`
	Metadata  string `db:"metadata"`
	MaxNodes  int    `db:"max_nodes"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
	DeletedAt string `db:"deleted_at"`
}

func (r Runtime) TableName() string {
	return "runtimes"
}

func (r Runtime) GetColumns() []string {
	return database.GenericGetColumns(r)
}

func (r Runtime) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(r)
}

func (r Runtime) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(r)
}

func (r Runtime) GetColumnType(column string) string {
	return database.GenericGetColumnType(r, column)
}
