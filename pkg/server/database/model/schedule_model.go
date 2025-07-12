package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type ScheduleModel struct {
	ID        int    `db:"id" sql:"INTEGER PRIMARY KEY AUTOINCREMENT"`
	Type      string `db:"type"`
	Code      string `db:"code"`
	Namespace string `db:"namespace"`
}

func (ScheduleModel) TableName() string {
	return "schedules"
}

func (w ScheduleModel) GetColumns() []string {
	return database.GenericGetColumns(w)
}

func (w ScheduleModel) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(w)
}

func (w ScheduleModel) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(w)
}

func (w ScheduleModel) GetColumnType(column string) string {
	return database.GenericGetColumnType(w, column)
}
