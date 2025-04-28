package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type Runtime struct {
	Name      string `db:"name" sql:"PRIMARY KEY AUTOINCREMENT"`
	Status    int    `db:"status"`
	Metadata  string `db:"metadata"`
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
