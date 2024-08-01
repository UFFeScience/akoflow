package main

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/engine/garbagecollector"
	"github.com/ovvesley/akoflow/pkg/server/engine/monitor"
)
import "github.com/ovvesley/akoflow/pkg/server/engine/worker"
import "github.com/ovvesley/akoflow/pkg/server/engine/orchestrator"
import "github.com/ovvesley/akoflow/pkg/server/engine/httpserver"

func main() {

	config.SetupEnv()

	config.App()

	go worker.StartWorker()
	go orchestrator.StartOrchestrator()
	go monitor.StartMonitor()
	go garbagecollector.StartGarbageCollector()
	httpserver.StartServer()

}
