package manager

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/parser"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/create_workflow_in_database_service"
)

var namespace = "k8science-cluster-manager"

var cont = 0

func DeployWorkflow(workflow workflow.Workflow) {
	jobs := parser.WorkflowToJobK8sService(workflow)

	createWorkflowInDatabaseService := create_workflow_in_database_service.New()

	err := createWorkflowInDatabaseService.Create(workflow)

	if err != nil {
		return
	}

	for _, job := range jobs {

		println("Sending job to channel")
		println("Job name: " + job.Metadata.Name)

	}

}
