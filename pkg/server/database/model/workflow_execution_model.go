package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type WorkflowExecution struct {
	ID         int    `db:"id" sql:"INTEGER PRIMARY KEY AUTOINCREMENT"`
	Namespace  string `db:"namespace"`
	Name       string `db:"name"`
	WorkflowID int    `db:"workflow_id"`
	Status     string `db:"status"`
	Runtime    string `db:"runtime"`
	CreatedAt  string `db:"created_at"`
	UpdatedAt  string `db:"updated_at"`
	DeletedAt  string `db:"deleted_at"`
}

func (WorkflowExecution) TableName() string {
	return "workflow_executions"
}

func (w WorkflowExecution) GetColumns() []string {
	return database.GenericGetColumns(w)
}

func (w WorkflowExecution) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(w)
}

func (w WorkflowExecution) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(w)
}

func (w WorkflowExecution) GetColumnType(column string) string {
	return database.GenericGetColumnType(w, column)
}
