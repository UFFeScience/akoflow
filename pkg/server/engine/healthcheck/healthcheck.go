package healthcheck

import (
	"time"

	"github.com/ovvesley/akoflow/pkg/server/config"
)

type HealthCheck struct {
	State int
}

func New() *HealthCheck {
	return &HealthCheck{
		State: 0,
	}
}

func (w *HealthCheck) StartHealthCheck() {

	config.App().Logger.Info("Healthcheck is running")

	envVarRuntimes := config.App().EnvVars.EnvVarByRuntime

	for runtime, envVars := range envVarRuntimes {
		config.App().Repository.RuntimeRepository.CreateOrUpdate(runtime, 1, envVars)
	}

	time.Sleep(5 * time.Second)

}
