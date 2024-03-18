package logs_repository

import "github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"

type ParamsLogsCreate struct {
	LogsDatabase LogsDatabase
}

func (l *LogsRepository) Create(params ParamsLogsCreate) error {

	database := connector.Database{}
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
