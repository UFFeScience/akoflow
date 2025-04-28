package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type PreActivity struct {
	ID                int    `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	ActivityID        int    `db:"activity_id"`
	WorkflowID        int    `db:"workflow_id"`
	Namespace         string `db:"namespace"`
	Name              string `db:"name"`
	ResourceK8sBase64 string `db:"resource_k8s_base64"` // Corrigido: era "TXT"
	Status            int    `db:"status"`
	Log               string `db:"log"`
}

func (PreActivity) TableName() string {
	return "pre_activities"
}

func (p PreActivity) GetColumns() []string {
	return database.GenericGetColumns(p)
}

func (p PreActivity) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(p)
}
