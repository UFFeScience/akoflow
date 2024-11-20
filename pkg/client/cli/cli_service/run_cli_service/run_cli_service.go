package run_cli_service

import (
	"flag"
	"os"

	"github.com/ovvesley/akoflow/pkg/client/connector/server_connector"
	"github.com/ovvesley/akoflow/pkg/client/services/dispatch_to_server_run_workflow_service"
	"github.com/ovvesley/akoflow/pkg/client/services/flag_validator_service"
)

type RunCliService struct {
}

func New() *RunCliService {
	return &RunCliService{}
}

func (r *RunCliService) Run() {

	host := flag.String("host", "localhost", "host")
	port := flag.String("port", "8080", "port")
	fileYaml := flag.String("file", "", "file")
	flag.CommandLine.Parse(os.Args[2:])

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

	dispatchToServerRunWorkflowService := dispatch_to_server_run_workflow_service.New(server_connector.New())

	dispatchToServerRunWorkflowService.
		SetHost(*host).
		SetPort(*port).
		SetFile(*fileYaml).
		Run()
}
