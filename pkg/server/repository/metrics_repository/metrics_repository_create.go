package metrics_repository

import "github.com/ovvesley/akoflow/pkg/server/repository"

type ParamsMetricsCreate struct {
	MetricsDatabase MetricsDatabase
}

func (m *MetricsRepository) Create(params ParamsMetricsCreate) error {

	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"INSERT INTO "+m.tableName+" (activity_id, cpu, memory, window, timestamp) VALUES (?, ?, ?, ?, ?)",
		params.MetricsDatabase.ActivityId, params.MetricsDatabase.Cpu, params.MetricsDatabase.Memory, params.MetricsDatabase.Window, params.MetricsDatabase.Timestamp)

	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}
