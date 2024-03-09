package orchestrator

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/orchestrate_workflow_service"
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
	workflows, _ := getPendingWorkflowService.GetPendingWorkflows()

	dispatchToWorkerActivityService := orchestrate_workflow_service.New()
	dispatchToWorkerActivityService.Orchestrate(workflows)

}
