package monitor

import (
	"time"

	"github.com/ovvesley/akoflow/pkg/server/services/monitor_change_workflow_service"
	"github.com/ovvesley/akoflow/pkg/server/services/monitor_collect_metrics_service"
)

const TimeToUpdateSeconds = 1

func StartMonitor() {
	for {
		handleMonitor()
		time.Sleep(TimeToUpdateSeconds * time.Second)
		println("Monitor is Listening...")

	}
}

func handleMonitor() {
	monitorChangeWorkflowService := monitor_change_workflow_service.New()
	monitorChangeWorkflowService.MonitorChangeWorkflow()

	monitorCollectMetricsService := monitor_collect_metrics_service.New()
	monitorCollectMetricsService.CollectMetrics()
}
