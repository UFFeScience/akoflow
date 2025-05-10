package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
)

type HealthCheckRuntimeK8sService struct {
	k8sConnector      connector_k8s.IConnector
	runtimeRepository runtime_repository.IRuntimeRepository
}

func NewHealthCheckRuntimeK8sService() *HealthCheckRuntimeK8sService {
	return &HealthCheckRuntimeK8sService{
		k8sConnector:      config.App().Connector.K8sConnector,
		runtimeRepository: config.App().Repository.RuntimeRepository,
	}
}
func (h *HealthCheckRuntimeK8sService) HealthCheck(runtime string) bool {

	// Check if the runtime is registered
	runtimeEntity, err := h.runtimeRepository.GetByName(runtime)
	if err != nil {
		config.App().Logger.Infof("WORKER: Runtime not found %s", runtime)
		return false
	}

	response := h.k8sConnector.Healthz(runtimeEntity).Healthz()

	if !response.Success {
		h.runtimeRepository.UpdateStatus(runtimeEntity, runtime_repository.STATUS_NOT_READY)
		config.App().Logger.Infof("WORKER: Health check failed for runtime %s", runtime)
		return false
	}

	h.runtimeRepository.UpdateStatus(runtimeEntity, runtime_repository.STATUS_READY)
	config.App().Logger.Infof("WORKER: Health check passed for runtime %s", runtime)

	return true
}
