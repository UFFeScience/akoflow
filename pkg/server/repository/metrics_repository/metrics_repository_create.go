package metrics_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/repository"
)

type ParamsMetricsCreate struct {
	MetricsDatabase MetricsDatabase
}

func (m *MetricsRepository) Create(params ParamsMetricsCreate) error {
	db := repository.GetInstance()

	md := params.MetricsDatabase

	query := fmt.Sprintf(
		"INSERT INTO %s (activity_id, cpu, memory, window, timestamp) VALUES (%d, '%s', '%s', '%s', '%s')",
		m.tableName,
		md.ActivityId,
		md.Cpu,
		md.Memory,
		md.Window,
		md.Timestamp,
	)

	_, err := db.Exec(query)
	return err
}
