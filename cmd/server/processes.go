package main

import (
	"context"
)

func (a *application) startEventLoop(ctx context.Context) {
	go func() {
		if err := a.eventLoop.Run(ctx); err != nil {
			a.log.Error("Event loop stopped:", err)
		}
	}()
}
