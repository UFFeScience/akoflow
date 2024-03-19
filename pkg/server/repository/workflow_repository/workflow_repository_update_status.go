package workflow_repository

import "github.com/ovvesley/scik8sflow/pkg/server/repository"

func (w *WorkflowRepository) UpdateStatus(id int, status int) error {
	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE "+w.tableName+" SET status = ? WHERE ID = ?", status, id)
	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	return nil
}
