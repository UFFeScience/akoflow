package manager

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/parser"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/workflow"
)

var namespace = "k8science-cluster-manager"

var cont = 0

func DeployWorkflow(workflow workflow.Workflow) {
	jobs := parser.WorkflowToJobK8sService(workflow)

	workflowRepository := workflow_repository.New()

	err := workflowRepository.Create(namespace, workflow)
	if err != nil {
		return
	}

	for _, job := range jobs {

		println("Sending job to channel")
		println("Job name: " + job.Metadata.Name)

	}

}
