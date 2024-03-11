package main

import (
	"flag"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/client/services/dispatch_to_server_run_workflow_service"
)

func main() {
	host := flag.String("host", "localhost", "host")
	port := flag.String("port", "8080", "port")
	fileYaml := flag.String("file", "", "file")

	flag.Parse()

	if *fileYaml == "" {
		println("file is required")
		return
	}

	dispatchToServerRunWorkflowService := dispatch_to_server_run_workflow_service.New()

	dispatchToServerRunWorkflowService.SetHost(*host)
	dispatchToServerRunWorkflowService.SetPort(*port)
	dispatchToServerRunWorkflowService.SetFile(*fileYaml)

	dispatchToServerRunWorkflowService.Run()

}
