package main

import (
	"context"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/installationhealth"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
)

func buildInstallationHealth(storage persistence, settings config.Settings) *installationhealth.Service {
	checks := []installationhealth.Probe{
		{ID: "database", Name: "Instance database", Action: "Review instance data management and available backups.",
			Check: func(ctx context.Context) installationhealth.Observation {
				return installationhealth.Database(ctx, storage.analytics)
			}},
		{ID: "storage", Name: "Artifact storage", Action: "Verify the configured artifact volume and its permissions.",
			Check: func(ctx context.Context) installationhealth.Observation {
				return installationhealth.Storage(ctx, settings.ArtifactStoreRoot, storage.readOnly)
			}},
		{ID: "docker", Name: "Docker", Action: "Start Docker and check that the daemon can access its socket.",
			Expensive: true, Check: installationhealth.Docker},
		{ID: "buildkit", Name: "BuildKit", Action: "Start the configured BuildKit service and verify its worker endpoint.",
			Expensive: true, Check: func(ctx context.Context) installationhealth.Observation {
				return installationhealth.BuildKit(ctx, settings.Buildctl)
			}},
	}
	return installationhealth.New(checks, storage.environments.InstallationConnectionHealth, 3*time.Second,
		max(15*time.Minute, 2*settings.ConnectionCheckInterval))
}
