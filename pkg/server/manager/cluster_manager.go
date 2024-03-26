package manager

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/services/create_workflow_in_database_service"
)

var namespace = "scik8sflow"

var cont = 0

func DeployWorkflow(workflow workflow.Workflow) {

	createWorkflowInDatabaseService := create_workflow_in_database_service.New()

	err := createWorkflowInDatabaseService.Create(workflow)

	if err != nil {
		return
	}
}
