package main

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/garbagecollector"
	"github.com/ovvesley/akoflow/pkg/server/httpserver"
	"github.com/ovvesley/akoflow/pkg/server/monitor"
	"github.com/ovvesley/akoflow/pkg/server/orchestrator"
	"github.com/ovvesley/akoflow/pkg/server/worker"
)

func main() {

	config.SetupEnv()

	go worker.StartWorker()
	go orchestrator.StartOrchestrator()
	go monitor.StartMonitor()

	go garbagecollector.StartGarbageCollector()

	httpserver.StartServer()

}
