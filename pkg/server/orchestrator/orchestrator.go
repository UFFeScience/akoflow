package orchestrator

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_pending_workflow_service"
	"time"
)

const TimeToUpdateSeconds = 5

func StartOrchestrator() {

	for {
		handleOrchestrator()
		time.Sleep(TimeToUpdateSeconds * time.Second)
	}

}

func handleOrchestrator() {
	getPendingWorkflowService := get_pending_workflow_service.New()
	workflows, err := getPendingWorkflowService.GetPendingWorkflows()
	if err != nil {
		return
	}
	println("Orchestrator: ", workflows)
}
