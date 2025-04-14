package workflow_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *WorkflowRepository) UpdateStatus(id int, status int) error {
	db := repository.GetInstance()

	query := fmt.Sprintf("UPDATE %s SET status = %d WHERE id = %d", w.tableName, status, id)

	_, err := db.Exec(query)
	return err
}
