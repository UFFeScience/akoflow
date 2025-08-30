package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type ActivitySchedule struct {
	ID             int     `db:"id" sql:"INTEGER PRIMARY KEY AUTOINCREMENT"`
	WorkflowID     int     `db:"workflow_id"`
	ActivityID     int     `db:"activity_id"`
	NodeName       string  `db:"node_name"`
	ScheduleName   string  `db:"schedule_name"`
	CpuRequired    float64 `db:"cpu_required"`
	MemoryRequired float64 `db:"memory_required"`
	Metadata       string  `db:"metadata"` // {"cpu", "memory", "score"}
	CreatedAt      string  `db:"created_at"`
}

func (ActivitySchedule) TableName() string {
	return "activities_schedules"
}

func (a ActivitySchedule) GetColumns() []string {
	return database.GenericGetColumns(a)
}

func (a ActivitySchedule) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(a)
}

func (a ActivitySchedule) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(a)
}

func (a ActivitySchedule) GetColumnType(column string) string {
	return database.GenericGetColumnType(a, column)
}
