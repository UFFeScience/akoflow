package main

import (
	"fmt"
	"os"

	"github.com/UFFeScience/akoflow-other-algorithms/internal/command"
	"github.com/UFFeScience/akoflow-other-algorithms/internal/planner"
)

func main() {
	if err := command.Run(planner.MemoryAware, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "memory-plan:", err)
		os.Exit(1)
	}
}
