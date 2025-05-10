package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type Activity struct {
	ID                int    `db:"id" sql:"INTEGER PRIMARY KEY AUTOINCREMENT"`
	WorkflowID        int    `db:"workflow_id"`
	Namespace         string `db:"namespace"`
	Name              string `db:"name"`
	Image             string `db:"image"`
	Runtime           string `db:"runtime"`
	ResourceK8sBase64 string `db:"resource_k8s_base64"`
	Status            int    `db:"status"`
	ProcID            string `db:"proc_id"`
	CreatedAt         string `db:"created_at"`
	StartedAt         string `db:"started_at"`
	FinishedAt        string `db:"finished_at"`
}

func (Activity) TableName() string {
	return "activities"
}

func (a Activity) GetColumns() []string {
	return database.GenericGetColumns(a)
}

func (a Activity) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(a)
}

func (a Activity) GetClausulePrimaryKey() string {
	return database.GenericGetClausulePrimaryKey(a)
}

func (a Activity) GetColumnType(column string) string {
	return database.GenericGetColumnType(a, column)
}
