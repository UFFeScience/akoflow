package main

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/engine/garbagecollector"
	"github.com/ovvesley/akoflow/pkg/server/engine/healthcheck"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver"
	"github.com/ovvesley/akoflow/pkg/server/engine/monitor"
	"github.com/ovvesley/akoflow/pkg/server/engine/orchestrator"
	"github.com/ovvesley/akoflow/pkg/server/engine/worker"
)

func main() {

	config.SetupEnv()

	config.App().Logger.Info("Starting Akoflow Server")

	go healthcheck.New().StartHealthCheck()
	go worker.New().StartWorker()
	go orchestrator.StartOrchestrator()
	go monitor.StartMonitor()
	go garbagecollector.StartGarbageCollector()

	httpserver.StartServer()

}
