package model

import "github.com/ovvesley/akoflow/pkg/server/utils/utils_model"

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
	return utils_model.GenericGetColumns(w)
}

func (w Workflow) GetPrimaryKey() string {
	return utils_model.GenericGetPrimaryKey(w)
}
