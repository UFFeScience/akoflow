package main

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/httpserver"
	"github.com/ovvesley/akoflow/pkg/server/orchestrator"
	"github.com/ovvesley/akoflow/pkg/server/worker"
)

func main() {

	config.SetupEnv()

	app := config.App()

	go worker.StartWorker(app)
	go orchestrator.StartOrchestrator(app)
	//go monitor.StartMonitor()
	//
	//go garbagecollector.StartGarbageCollector()
	//
	httpserver.StartServer()

}
