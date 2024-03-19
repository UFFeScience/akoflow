package connector

import (
	"crypto/tls"
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_deployment_k8s"
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_job_k8s"
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_metrics_k8s"
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_namespace_k8s"
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_pod_k8s"
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_pvc_k8s"
	"net/http"
)

type Connector struct {
	client *http.Client
}

type IConnector interface {
	Namespace() connector_namespace_k8s.IConnectorNamespace
	Pod() connector_pod_k8s.IConnectorPod
	Job() connector_job_k8s.IConnectorJob
	Deployment() connector_deployment_k8s.IConnectorDeployment
	Metrics() connector_metrics_k8s.IConnectorMetrics
	PersistentVolumeClain() connector_pvc_k8s.IConnectorPvc
}

func NewClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

func New() IConnector {
	client := NewClient()
	return &Connector{client: client}
}

func (c *Connector) Namespace() connector_namespace_k8s.IConnectorNamespace {
	return connector_namespace_k8s.New()
}

func (c *Connector) Pod() connector_pod_k8s.IConnectorPod {
	return connector_pod_k8s.New()
}

func (c *Connector) Job() connector_job_k8s.IConnectorJob {
	return connector_job_k8s.New()
}

func (c *Connector) Deployment() connector_deployment_k8s.IConnectorDeployment {
	return connector_deployment_k8s.New()
}

func (c *Connector) Metrics() connector_metrics_k8s.IConnectorMetrics {
	return connector_metrics_k8s.New()
}

func (c *Connector) PersistentVolumeClain() connector_pvc_k8s.IConnectorPvc {
	return connector_pvc_k8s.New()
}
