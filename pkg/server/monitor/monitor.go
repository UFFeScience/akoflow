package monitor

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/monitor_change_workflow_service"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/monitor_collect_metrics_service"
	"time"
)

const TimeToUpdateSeconds = 1

func StartMonitor() {
	for {
		handleMonitor()
		time.Sleep(TimeToUpdateSeconds * time.Second)

	}
}

func handleMonitor() {
	monitorChangeWorkflowService := monitor_change_workflow_service.New()
	monitorChangeWorkflowService.MonitorChangeWorkflow()

	monitorCollectMetricsService := monitor_collect_metrics_service.New()
	monitorCollectMetricsService.CollectMetrics()
}
