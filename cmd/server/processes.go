package main

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/provider/kubernetes"
)

func buildHistoryCleaner(settings config.Settings, api kubernetes.API) (*kubernetes.HistoryCleaner, error) {
	if api == nil || !settings.KubernetesCleanupEnabled {
		return nil, nil
	}
	return kubernetes.NewHistoryCleaner(api, settings.DefaultNamespace, settings.KubernetesHistoryRetention)
}

func (a *application) startEventLoop(ctx context.Context) {
	go func() {
		if err := a.eventLoop.Run(ctx); err != nil {
			a.log.Error("Event loop stopped:", err)
		}
	}()
}

func (a *application) startHistoryCleanup(ctx context.Context) {
	if a.historyCleaner == nil {
		return
	}
	go func() {
		err := a.historyCleaner.Run(ctx, a.settings.KubernetesCleanupInterval, a.reportCleanup)
		if err != nil && err != context.Canceled {
			a.log.Error("Kubernetes history cleaner stopped:", err)
		}
	}()
}

func (a *application) reportCleanup(result kubernetes.CleanupResult, err error) {
	if err != nil {
		a.log.Error("Kubernetes history cleanup failed:", err)
		return
	}
	if result.JobsDeleted > 0 || result.PodsDeleted > 0 {
		a.log.Infof("Kubernetes history cleanup removed %d jobs and %d pods",
			result.JobsDeleted, result.PodsDeleted)
	}
}
