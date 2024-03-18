package activities_repository

import "github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"

func (w *ActivityRepository) UpdateStatus(id int, status int) error {
	database := connector.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE "+w.tableNameActivity+" SET status = ? WHERE ID = ?", status, id)
	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}
