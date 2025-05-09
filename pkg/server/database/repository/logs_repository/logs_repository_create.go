package logs_repository

import "github.com/ovvesley/akoflow/pkg/server/database/repository"

type ParamsLogsCreate struct {
	LogsDatabase LogsDatabase
}

func (l *LogsRepository) Create(params ParamsLogsCreate) error {

	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec(
		"INSERT INTO "+l.tableName+" (activity_id, logs) VALUES (?, ?)",
		params.LogsDatabase.ActivityId, params.LogsDatabase.Logs)

	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil

}
