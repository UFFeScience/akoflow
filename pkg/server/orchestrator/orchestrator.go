package orchestrator

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/services/get_pending_workflow_service"
	"github.com/ovvesley/akoflow/pkg/server/services/orchestrate_workflow_service"
)

const TimeToUpdateSeconds = 1

func StartOrchestrator(app config.AppContainer) {

	for {
		handleOrchestrator(app)
		time.Sleep(TimeToUpdateSeconds * time.Second)
		println("Orchestrator is Listening...")
	}

}

func handleOrchestrator(app config.AppContainer) {
	getPendingWorkflowService := get_pending_workflow_service.New()
	workflows, _ := getPendingWorkflowService.GetPendingWorkflows()

	dispatchToWorkerActivityService := orchestrate_workflow_service.New()
	dispatchToWorkerActivityService.Orchestrate(workflows)

}
