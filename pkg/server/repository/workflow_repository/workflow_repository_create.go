package workflow_repository

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository"
)

func (w *WorkflowRepository) Create(namespace string, workflow workflow_entity.Workflow) (int, error) {
	db := repository.GetInstance()

	rawWorkflow := workflow.GetBase64Workflow()

	query := fmt.Sprintf(
		"INSERT INTO %s (namespace, runtime, name, raw_workflow, status) VALUES ('%s', '%s', '%s', '%s', %d)",
		w.tableName,
		namespace,
		workflow.Spec.Runtime,
		workflow.Name,
		rawWorkflow,
		StatusCreated,
	)

	resp, err := db.Exec(query)
	if err != nil {
		return 0, err
	}

	// Pega o last_insert_id do rqlite
	results, ok := resp["results"].([]interface{})
	if !ok || len(results) == 0 {
		return 0, fmt.Errorf("invalid response from rqlite")
	}
	if len(results) > 0 {
		res := results[0].(map[string]interface{})
		if lastID, ok := res["last_insert_id"].(float64); ok {
			return int(lastID), nil
		}
	}

	return 0, nil
}
