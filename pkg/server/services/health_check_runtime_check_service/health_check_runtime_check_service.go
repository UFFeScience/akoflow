package health_check_runtime_check_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/runtimes"
)

type HealthCheckRuntimeCheckService struct {
	State             int
	runtimeRepository runtime_repository.IRuntimeRepository
}

func New() *HealthCheckRuntimeCheckService {
	return &HealthCheckRuntimeCheckService{
		State: 0,
	}
}
func NewHealthCheckRuntimeCheckService() *HealthCheckRuntimeCheckService {
	return &HealthCheckRuntimeCheckService{
		State:             0,
		runtimeRepository: config.App().Repository.RuntimeRepository,
	}
}

func (w *HealthCheckRuntimeCheckService) Handle(runtimeName string) {

	runtimes.
		GetRuntimeInstance(runtimeName).
		HealthCheck()

}
