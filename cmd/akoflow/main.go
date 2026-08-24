package main

import (
	"os"

	"github.com/UFFeScience/akoflow/internal/cli/command"
)

func main() {

	name := os.Args[1]

	command.New(name).Run()

}
