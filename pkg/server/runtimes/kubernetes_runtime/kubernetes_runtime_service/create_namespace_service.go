package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector"
)

type CreateNamespaceService struct {
	connector connector.IConnector
}

type ParamsNewCreateNamespaceService struct {
	Connector connector.IConnector
}

func NewCreateNamespaceService() CreateNamespaceService {
	return CreateNamespaceService{
		connector: config.App().Connector.K8sConnector,
	}
}

func (r *CreateNamespaceService) GetOrCreateNamespace(namespace string) (string, error) {
	return r.handleGetOrCreateNamespace(namespace)
}

func (r *CreateNamespaceService) handleGetOrCreateNamespace(namespace string) (string, error) {
	response, err := r.connector.Namespace().GetNamespace(namespace)

	if err != nil {
		println("Namespace not found")
		return r.handleCreateNamespace(namespace)
	}

	return response.Metadata.Name, nil
}

func (r *CreateNamespaceService) handleCreateNamespace(namespace string) (string, error) {
	ns, err := r.connector.Namespace().CreateNamespace(namespace)

	if err != nil {
		println("Error creating namespace")
		return "", err
	}

	return ns.Metadata.Name, nil

}
