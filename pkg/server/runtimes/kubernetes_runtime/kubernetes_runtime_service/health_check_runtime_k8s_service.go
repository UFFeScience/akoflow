package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/node_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
)

type HealthCheckRuntimeK8sService struct {
	k8sConnector      connector_k8s.IConnector
	runtimeRepository runtime_repository.IRuntimeRepository
	nodeRepository    node_repository.INodeRepository
}

func NewHealthCheckRuntimeK8sService() *HealthCheckRuntimeK8sService {
	return &HealthCheckRuntimeK8sService{
		k8sConnector:      config.App().Connector.K8sConnector,
		runtimeRepository: config.App().Repository.RuntimeRepository,
		nodeRepository:    config.App().Repository.NodeRepository,
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

func (h *HealthCheckRuntimeK8sService) DiscoverNode(runtime string) bool {
	runtimeEntity, err := h.runtimeRepository.GetByName(runtime)
	if err != nil {
		config.App().Logger.Infof("WORKER: Runtime not found %s", runtime)
		return false
	}

	response := h.k8sConnector.Nodes(runtimeEntity).ListNodes()

	if !response.Success {
		config.App().Logger.Infof("WORKER: Node discovery failed for runtime %s", runtime)
		return false
	}

	for _, node := range response.Data {
		node := model.Node{
			Name:         node.Name,
			Runtime:      runtime,
			Status:       node_repository.STATUS_READY,
			CPUUsage:     0.0, // Assuming initial CPU usage is 0
			CPUMax:       node.GetCpuMax(),
			MemoryUsage:  0.0, // Assuming initial memory usage is 0
			MemoryLimit:  node.GetNodeMemoryMax(),
			NetworkLimit: node.GetNodeNetworkMax(),
			NetworkUsage: 0.0, // Assuming initial network usage is 0
		}
		err := h.nodeRepository.CreateOrUpdate(runtime, node)
		if err != nil {
			config.App().Logger.Error("WORKER: Failed to create or update node %s for runtime %s: %v", node.Name, runtime, err)
			return false
		}

	}

	config.App().Logger.Infof("WORKER: Node discovery successful for runtime %s", runtime)
	return true

}
