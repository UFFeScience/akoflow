package manager

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/parser"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/workflow"
)

var namespace = "k8science-cluster-manager"

func DeployWorkflow(workflow workflow.Workflow) {
	jobs := parser.WorkflowToJobK8sService(workflow)

	c := connector.New()

	for _, job := range jobs {
		c.ApplyJob(namespace, job)
	}

}
