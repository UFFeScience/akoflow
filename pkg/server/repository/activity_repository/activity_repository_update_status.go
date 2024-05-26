package activity_repository

import "github.com/ovvesley/akoflow/pkg/server/repository"

func (w *ActivityRepository) UpdateStatus(id int, status int) error {
	database := repository.Database{}
	c := database.Connect()

	if status == StatusFinished {
		_, err := c.Exec("UPDATE "+w.tableNameActivity+" SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE ID = ?", status, id)
		if err != nil {
			return err
		}
	}

	if status == StatusRunning {
		_, err := c.Exec("UPDATE "+w.tableNameActivity+" SET status = ?, started_at = CURRENT_TIMESTAMP WHERE ID = ?", status, id)
		if err != nil {
			return err
		}
	}

	if status == StatusCreated {
		_, err := c.Exec("UPDATE "+w.tableNameActivity+" SET status = ? WHERE ID = ?", status, id)
		if err != nil {
			return err
		}
	}

	err := c.Close()

	if err != nil {
		return err
	}

	return nil
}
