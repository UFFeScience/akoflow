package model

import "github.com/ovvesley/akoflow/pkg/server/utils/utils_model"

type ActivityDependency struct {
	ID               int `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	WorkflowID       int `db:"workflow_id"`
	ActivityID       int `db:"activity_id"`
	DependOnActivity int `db:"depend_on_activity"`
}

func (ActivityDependency) TableName() string {
	return "activities_dependencies"
}

func (a ActivityDependency) GetColumns() []string {
	return utils_model.GenericGetColumns(a)
}

func (a ActivityDependency) GetPrimaryKey() string {
	return utils_model.GenericGetPrimaryKey(a)
}
