package monitor

import (
	"github.com/ovvesley/scik8sflow/pkg/server/services/monitor_change_workflow_service"
	"time"
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

	//monitorCollectMetricsService := monitor_collect_metrics_service.New()
	//monitorCollectMetricsService.CollectMetrics()
}
