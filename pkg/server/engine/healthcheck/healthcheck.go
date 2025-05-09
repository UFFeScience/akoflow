package healthcheck

import (
	"time"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/services/health_check_runtime_check_service"
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

	config.App().Logger.Info("Starting healthcheck")
	envVarRuntimes := config.App().EnvVars.EnvVarByRuntime

	for runtime, envVars := range envVarRuntimes {
		config.App().Repository.RuntimeRepository.CreateOrUpdate(runtime, 0, envVars)
	}

	for {

		config.App().Logger.Info("Running healthcheck")
		envVarRuntimes := config.App().EnvVars.EnvVarByRuntime

		for runtime := range envVarRuntimes {
			healthCheckRuntimeCheckService := health_check_runtime_check_service.NewHealthCheckRuntimeCheckService()
			healthCheckRuntimeCheckService.Handle(runtime)
		}

		time.Sleep(5 * time.Second)
	}

}
