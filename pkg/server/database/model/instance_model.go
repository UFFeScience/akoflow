package model

import "github.com/ovvesley/akoflow/pkg/server/database"

type Instance struct {
	ID        int    `db:"id" sql:"PRIMARY KEY AUTOINCREMENT"`
	Name      string `db:"name"`
	Status    string `db:"status"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
	DeletedAt string `db:"deleted_at"`
}

func (w Instance) TableName() string {
	return "instances"
}

func (w Instance) GetColumns() []string {
	return database.GenericGetColumns(w)
}

func (w Instance) GetPrimaryKey() string {
	return database.GenericGetPrimaryKey(w)
}
