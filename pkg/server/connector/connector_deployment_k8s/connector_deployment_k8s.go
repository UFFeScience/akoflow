package connector_deployment_k8s

import (
	"crypto/tls"
	"net/http"
)

type ConnectorDeploymentK8s struct {
	client *http.Client
}

type IConnectorDeployment interface {
	ListDeployments()
}

func New() IConnectorDeployment {
	return &ConnectorDeploymentK8s{
		client: newClient(),
	}
}

func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

func (c *ConnectorDeploymentK8s) ListDeployments() {
	//TODO implement me
	panic("implement me")
}
