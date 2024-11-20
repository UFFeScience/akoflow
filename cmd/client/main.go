package main

import (
	"os"

	"github.com/ovvesley/akoflow/pkg/client/cli/cli_service"
)

func main() {

	command := os.Args[1]

	cliService := cli_service.New(command)
	cliService.Run()

}
