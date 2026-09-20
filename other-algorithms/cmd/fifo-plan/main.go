package main

import (
	"fmt"
	"os"

	"github.com/UFFeScience/akoflow-other-algorithms/internal/command"
	"github.com/UFFeScience/akoflow-other-algorithms/internal/planner"
)

func main() {
	if err := command.Run(planner.FIFO, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fifo-plan:", err)
		os.Exit(1)
	}
}
