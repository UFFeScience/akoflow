package model

import (
	"github.com/ovvesley/akoflow/pkg/server/database"
)

type Workflow struct {
	ID          int    `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	Namespace   string `db:"namespace"`
	Name        string `db:"name"`
	RawWorkflow string `db:"raw_workflow"`
	Status      string `db:"status"`
	Runtime     string `db:"runtime"`
	CreatedAt   string `db:"created_at"`
	UpdatedAt   string `db:"updated_at"`
	DeletedAt   string `db:"deleted_at"`
}

func (w Workflow) TableName() string {
	return "workflows"
}

func (w Workflow) GetColumns() []string {
	return database.GenericGetColumns(w)
}

func (w Workflow) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(w)
}
