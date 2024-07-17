package connector

import (
	"crypto/tls"
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/connector/connector_cluster_role"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_cluster_role_binding"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_deployment_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_job_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_metrics_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_namespace_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_pod_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_pvc_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_role"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_role_binding"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_service"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_service_account"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_storage_class"
)

type Connector struct {
	client *http.Client
}

type IConnector interface {
	// Namespace connects to the Kubernetes API to manage namespaces.
	// API Endpoint: /api/v1/namespaces
	Namespace() connector_namespace_k8s.IConnectorNamespace

	// Pod connects to the Kubernetes API to manage pods.
	// API Endpoint: /api/v1/namespaces/{namespace}/pods
	Pod() connector_pod_k8s.IConnectorPod

	// Job connects to the Kubernetes API to manage jobs.
	// API Endpoint: /apis/batch/v1/namespaces/{namespace}/jobs
	Job() connector_job_k8s.IConnectorJob

	// Deployment connects to the Kubernetes API to manage deployments.
	// API Endpoint: /apis/apps/v1/namespaces/{namespace}/deployments
	Deployment() connector_deployment_k8s.IConnectorDeployment

	// Metrics connects to the Kubernetes API to retrieve metrics.
	// API Endpoint: /apis/metrics.k8s.io/v1beta1
	Metrics() connector_metrics_k8s.IConnectorMetrics

	// PersistentVolumeClain PersistentVolumeClaim connects to the Kubernetes API to manage persistent volume claims.
	// API Endpoint: /api/v1/namespaces/{namespace}/persistentvolumeclaims
	PersistentVolumeClain() connector_pvc_k8s.IConnectorPvc

	// ClusterRole connects to the Kubernetes API to manage cluster roles.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/clusterroles
	ClusterRole() connector_cluster_role.IConnectorClusterRole

	// ClusterRoleBinding connects to the Kubernetes API to manage cluster role bindings.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/clusterrolebindings
	ClusterRoleBinding() connector_cluster_role_binding.IConnectorClusterRoleBinding

	// Role connects to the Kubernetes API to manage roles.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/namespaces/{namespace}/roles
	Role() connector_role.IConnectorRole

	// RoleBinding connects to the Kubernetes API to manage role bindings.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/namespaces/{namespace}/rolebindings
	RoleBinding() connector_role_binding.IConnectorRoleBinding

	// Service connects to the Kubernetes API to manage services.
	// API Endpoint: /api/v1/namespaces/{namespace}/services
	Service() connector_service.IConnectorService

	// ServiceAccount connects to the Kubernetes API to manage service accounts.
	// API Endpoint: /api/v1/namespaces/{namespace}/serviceaccounts
	ServiceAccount() connector_service_account.IConnectorServiceAccount

	// StorageClass connects to the Kubernetes API to manage storage classes.
	// API Endpoint: /apis/storage.k8s.io/v1/storageclasses
	StorageClass() connector_storage_class.IConnectorStorageClass
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

func (c *Connector) ClusterRole() connector_cluster_role.IConnectorClusterRole {
	return connector_cluster_role.New()
}

func (c *Connector) ClusterRoleBinding() connector_cluster_role_binding.IConnectorClusterRoleBinding {
	return connector_cluster_role_binding.New()
}

func (c *Connector) Role() connector_role.IConnectorRole {
	return connector_role.New()
}

func (c *Connector) RoleBinding() connector_role_binding.IConnectorRoleBinding {
	return connector_role_binding.New()
}

func (c *Connector) Service() connector_service.IConnectorService {
	return connector_service.New()
}

func (c *Connector) ServiceAccount() connector_service_account.IConnectorServiceAccount {
	return connector_service_account.New()
}

func (c *Connector) StorageClass() connector_storage_class.IConnectorStorageClass {
	return connector_storage_class.New()
}
