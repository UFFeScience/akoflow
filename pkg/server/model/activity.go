package model

import "github.com/ovvesley/akoflow/pkg/server/utils/utils_model"

type Activity struct {
	ID                int    `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	WorkflowID        int    `db:"workflow_id"`
	Namespace         string `db:"namespace"`
	Name              string `db:"name"`
	Image             string `db:"image"`
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
	return utils_model.GenericGetColumns(a)
}

func (a Activity) GetPrimaryKey() string {
	return utils_model.GenericGetPrimaryKey(a)
}
