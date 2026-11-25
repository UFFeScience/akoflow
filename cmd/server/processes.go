package main

import (
	"context"
	"time"
)

func (a *application) startEventLoop(ctx context.Context) {
	go func() {
		if err := a.eventLoop.Run(ctx); err != nil {
			a.log.Error("Event loop stopped:", err)
		}
	}()
}

func (a *application) dispatchExecutionSchedules(ctx context.Context) {
	if a.executionSchedules == nil {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if a.runtimeUpdate.Admit() {
			err := a.executionSchedules.Dispatch(ctx, time.Now().UTC())
			a.runtimeUpdate.Done()
			if err != nil && ctx.Err() == nil {
				a.log.Error("Dispatch execution schedules:", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
