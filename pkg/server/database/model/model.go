package model

type Model interface {
	TableName() string
	GetColumns() []string
	GetPrimaryKey() string
	GetClausulePrimaryKey() string
	GetColumnType(string) string
}
