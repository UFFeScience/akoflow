package create_namespace_service

import "github.com/ovvesley/scik8sflow/pkg/server/connector"

type CreateNamespaceService struct {
	connector connector.IConnector
}

type ParamsNewCreateNamespaceService struct {
	Connector connector.IConnector
}

func New(params ...ParamsNewCreateNamespaceService) CreateNamespaceService {
	if len(params) > 0 {
		return CreateNamespaceService{
			connector: params[0].Connector,
		}
	}
	return CreateNamespaceService{
		connector: connector.New(),
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
