package command

import (
	"flag"
	"os"

	"github.com/UFFeScience/akoflow/internal/cli/api/server_connector"
	"github.com/UFFeScience/akoflow/internal/cli/validation"
	cliworkflow "github.com/UFFeScience/akoflow/internal/cli/workflow"
)

type Run struct{}

func (r *Run) Run() {

	host := flag.String("host", "localhost", "host")
	port := flag.String("port", "8080", "port")
	fileYaml := flag.String("file", "", "file")
	flag.CommandLine.Parse(os.Args[2:])

	validator := validation.New()

	if !validator.ValidateFile(*fileYaml) {
		panic("Invalid file")
	}

	if !validator.ValidateHost(*host) {
		panic("Invalid host")
	}

	if !validator.ValidatePort(*port) {
		panic("Invalid port")
	}

	runner := cliworkflow.New(server_connector.New())

	runner.
		SetHost(*host).
		SetPort(*port).
		SetFile(*fileYaml)
	if err := runner.Run(); err != nil {
		panic(err)
	}
}
