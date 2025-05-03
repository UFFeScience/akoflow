package model

type Model interface {
	TableName() string
	GetColumns() []string
	GetPrimaryKey() string
}
