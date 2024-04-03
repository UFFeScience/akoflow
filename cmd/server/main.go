package main

import (
	"github.com/ovvesley/scik8sflow/pkg/server/config"
	"github.com/ovvesley/scik8sflow/pkg/server/httpserver"
	"github.com/ovvesley/scik8sflow/pkg/server/monitor"
	"github.com/ovvesley/scik8sflow/pkg/server/orchestrator"
	"github.com/ovvesley/scik8sflow/pkg/server/worker"
)

func main() {

	config.SetupEnv()

	go worker.StartWorker()
	go orchestrator.StartOrchestrator()
	go monitor.StartMonitor()

	httpserver.StartServer()

}
