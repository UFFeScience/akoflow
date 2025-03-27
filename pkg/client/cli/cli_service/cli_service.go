package cli_service

import (
	"github.com/ovvesley/akoflow/pkg/client/cli/cli_service/install_cli_service"
	"github.com/ovvesley/akoflow/pkg/client/cli/cli_service/run_cli_service"
)

type CliService interface {
	Run()
}

var cliServices = map[string]CliService{
	"run":     run_cli_service.New(),
	"install": install_cli_service.New(),
}

func New(cliService string) CliService {

	if _, ok := cliServices[cliService]; !ok {
		panic("Invalid command :: " + cliService + " ::")
	}

	return cliServices[cliService]
}
