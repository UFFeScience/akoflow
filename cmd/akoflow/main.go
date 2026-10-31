package main

import (
	"context"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/mcpbridge"
	"os"
	"os/signal"
	"syscall"

	"github.com/UFFeScience/akoflow/internal/cli/command"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := mcpbridge.RunCommand(ctx, os.Args[2:], os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: akoflow <run|mcp>")
		os.Exit(2)
	}

	name := os.Args[1]

	command.New(name).Run()

}
