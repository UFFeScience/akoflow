package main

import (
	"context"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/mcpbridge"
	"os"
	"os/signal"
	"syscall"

	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		if err := mcpbridge.RunCommand(ctx, os.Args[2:], os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	application, err := newApplication(ctx, config.Load(), logger.NewStdLogger())
	if err != nil {
		panic(err)
	}
	defer application.Close()

	if err := application.Run(ctx); err != nil {
		panic(err)
	}
}
