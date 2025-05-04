package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type CreateNamespaceService struct {
	connector connector_k8s.IConnector

	runtimeRepository runtime_repository.IRuntimeRepository
}

type ParamsNewCreateNamespaceService struct {
	Connector connector_k8s.IConnector
}

func NewCreateNamespaceService() CreateNamespaceService {
	return CreateNamespaceService{
		connector: config.App().Connector.K8sConnector,

		runtimeRepository: config.App().Repository.RuntimeRepository,
	}
}

func (r *CreateNamespaceService) GetOrCreateNamespace(wf workflow_entity.Workflow, namespace string) (string, error) {
	return r.handleGetOrCreateNamespace(wf, namespace)
}

func (r *CreateNamespaceService) handleGetOrCreateNamespace(wf workflow_entity.Workflow, namespace string) (string, error) {

	runtime, err := r.runtimeRepository.GetByName(wf.GetRuntimeId())
	if err != nil {
		return "", err
	}

	response, err := r.connector.Namespace(runtime).GetNamespace(namespace)

	if err != nil {
		println("Namespace not found")
		return r.handleCreateNamespace(wf, namespace)
	}

	return response.Metadata.Name, nil
}

func (r *CreateNamespaceService) handleCreateNamespace(wf workflow_entity.Workflow, namespace string) (string, error) {

	runtime, err := r.runtimeRepository.GetByName(wf.GetRuntimeId())
	if err != nil {
		return "", err
	}

	ns, err := r.connector.Namespace(runtime).CreateNamespace(namespace)

	if err != nil {
		println("Error creating namespace")
		return "", err
	}

	return ns.Metadata.Name, nil

}
