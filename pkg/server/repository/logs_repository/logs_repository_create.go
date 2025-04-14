package logs_repository

import (
	"fmt"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type ParamsLogsCreate struct {
	LogsDatabase LogsDatabase
}

func (l *LogsRepository) Create(params ParamsLogsCreate) error {
	db := repository.GetInstance()

	ld := params.LogsDatabase

	query := fmt.Sprintf(
		"INSERT INTO %s (activity_id, logs) VALUES (%d, '%s')",
		l.tableName,
		ld.ActivityId,
		escape(ld.Logs),
	)

	_, err := db.Exec(query)
	return err
}
func escape(s string) string {
	// Substitui aspas simples por duas aspas simples (padrão SQL)
	return strings.ReplaceAll(s, "'", "''")
}
