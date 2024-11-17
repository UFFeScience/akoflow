package main

import (
	"flag"

	"github.com/ovvesley/akoflow/pkg/client/services/dispatch_to_server_run_workflow_service"
	"github.com/ovvesley/akoflow/pkg/client/services/flag_validator_service"
)

func main() {
	host := flag.String("host", "localhost", "host")
	port := flag.String("port", "8080", "port")
	fileYaml := flag.String("file", "", "file")

	flag.Parse()

	flagValidatorService := flag_validator_service.New()

	if !flagValidatorService.ValidateFile(*fileYaml) {
		panic("Invalid file")
	}

	if !flagValidatorService.ValidateHost(*host) {
		panic("Invalid host")
	}

	if !flagValidatorService.ValidatePort(*port) {
		panic("Invalid port")
	}

	dispatchToServerRunWorkflowService := dispatch_to_server_run_workflow_service.New()

	dispatchToServerRunWorkflowService.
		SetHost(*host).
		SetPort(*port).
		SetFile(*fileYaml).
		Run()

}
