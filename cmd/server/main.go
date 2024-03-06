package main

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/httpserver"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/orchestrator"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/worker"
)

func main() {

	go worker.StartWorker()
	go orchestrator.StartOrchestrator()

	httpserver.StartServer()

}
