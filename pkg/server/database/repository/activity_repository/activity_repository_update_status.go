package activity_repository

import "github.com/ovvesley/akoflow/pkg/server/database/repository"

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

func (w *ActivityRepository) UpdateProcID(id int, pid string) error {
	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE "+w.tableNameActivity+" SET proc_id = ? WHERE ID = ?", pid, id)
	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}

func (w *ActivityRepository) UpdateNodeSelector(id int, nodeSelector string) error {
	database := repository.Database{}
	c := database.Connect()

	_, err := c.Exec("UPDATE "+w.tableNameActivity+" SET node_selector = ? WHERE ID = ?", nodeSelector, id)
	if err != nil {
		return err
	}

	err = c.Close()

	if err != nil {
		return err
	}

	return nil
}
