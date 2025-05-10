package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type ActivityDependency struct {
	ID               int `db:"id" sql:"INTEGER PRIMARY KEY AUTOINCREMENT"`
	WorkflowID       int `db:"workflow_id"`
	ActivityID       int `db:"activity_id"`
	DependOnActivity int `db:"depend_on_activity"`
}

func (ActivityDependency) TableName() string {
	return "activities_dependencies"
}

func (a ActivityDependency) GetColumns() []string {
	return database.GenericGetColumns(a)
}

func (a ActivityDependency) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(a)
}

func (a ActivityDependency) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(a)
}

func (a ActivityDependency) GetColumnType(column string) string {
	return database.GenericGetColumnType(a, column)
}
