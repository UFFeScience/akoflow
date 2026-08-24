package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := newApplication(ctx, config.Load(), logger.NewStdLogger())
	if err != nil {
		panic(err)
	}
	defer application.Close()

	if err := application.Run(ctx); err != nil {
		panic(err)
	}
}
