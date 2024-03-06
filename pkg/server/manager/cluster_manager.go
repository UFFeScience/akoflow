package manager

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/parser"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/workflow"
)

var namespace = "k8science-cluster-manager"

var cont = 0

func DeployWorkflow(workflow workflow.Workflow) {
	jobs := parser.WorkflowToJobK8sService(workflow)

	managerChannel := channel.GetInstance()

	for _, job := range jobs {
		dataToChannel := channel.DataChannel{
			Job:       job,
			Cont:      cont,
			Namespace: namespace,
		}
		println("Sending job to channel")
		managerChannel.WorfklowChannel <- dataToChannel
		cont++
	}

}
