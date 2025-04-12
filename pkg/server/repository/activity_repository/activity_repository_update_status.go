package activity_repository

import (
	"fmt"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *ActivityRepository) UpdateStatus(id int, status int) error {
	db := repository.GetInstance()

	var query string

	switch status {
	case StatusFinished:
		query = fmt.Sprintf(
			"UPDATE %s SET status = %d, finished_at = CURRENT_TIMESTAMP WHERE id = %d",
			w.tableNameActivity, status, id)
	case StatusRunning:
		query = fmt.Sprintf(
			"UPDATE %s SET status = %d, started_at = CURRENT_TIMESTAMP WHERE id = %d",
			w.tableNameActivity, status, id)
	case StatusCreated:
		query = fmt.Sprintf(
			"UPDATE %s SET status = %d WHERE id = %d",
			w.tableNameActivity, status, id)
	default:
		return fmt.Errorf("invalid status value: %d", status)
	}

	_, err := db.Exec(query)
	return err
}

func (w *ActivityRepository) UpdateProcID(id int, pid string) error {
	db := repository.GetInstance()

	query := fmt.Sprintf(
		"UPDATE %s SET proc_id = '%s' WHERE id = %d",
		w.tableNameActivity,
		escape(pid),
		id,
	)

	_, err := db.Exec(query)
	return err
}

func escape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
