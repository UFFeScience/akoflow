package workflow_execution_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
)

type WorkflowExecutionRepository struct {
	tableName string
}

const STATUS_RUNNING = 1
const STATUS_COMPLETED = 2
const STATUS_FAILED = 3
const STATUS_CANCELLED = 4

type IWorkflowExecutionRepository interface {
	CreateOrUpdate(workflowID string, status int, metadata map[string]string)
}

func New() IWorkflowExecutionRepository {

	database := repository.Database{}
	model := model.WorkflowExecution{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model)
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &WorkflowExecutionRepository{
		tableName: model.TableName(),
	}
}

func (wer *WorkflowExecutionRepository) CreateOrUpdate(workflowID string, status int, metadata map[string]string) {
	// Implementation for creating or updating a workflow execution entry
	// This would typically involve inserting or updating the record in the database
	// using the provided workflowID, status, and metadata.

	// Example implementation (pseudo-code):
	/*
		c := repository.Database.Connect()
		defer c.Close()

		query := fmt.Sprintf("INSERT INTO %s (workflow_id, status, metadata) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE status = ?, metadata = ?", wer.tableName)
		_, err := c.Exec(query, workflowID, status, metadata, status, metadata)
		if err != nil {
			log.Error("Failed to create or update workflow execution:", err)
		}
	*/
}
