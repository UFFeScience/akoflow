package connector_k8s

import (
	"crypto/tls"
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_cluster_role"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_cluster_role_binding"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_deployment_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_healthz"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_job_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_metrics_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_namespace_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_node_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_pod_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_pvc_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_role"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_role_binding"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_service"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_service_account"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_storage_class"
	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type Connector struct {
	client  *http.Client
	runtime *runtime_entity.Runtime
}

type IConnector interface {
	// Namespace connects to the Kubernetes API to manage namespaces.
	// API Endpoint: /api/v1/namespaces
	Namespace(*runtime_entity.Runtime) connector_namespace_k8s.IConnectorNamespace

	// Pod connects to the Kubernetes API to manage pods.
	// API Endpoint: /api/v1/namespaces/{namespace}/pods
	Pod(*runtime_entity.Runtime) connector_pod_k8s.IConnectorPod

	// Job connects to the Kubernetes API to manage jobs.
	// API Endpoint: /apis/batch/v1/namespaces/{namespace}/jobs
	Job(*runtime_entity.Runtime) connector_job_k8s.IConnectorJob

	// Deployment connects to the Kubernetes API to manage deployments.
	// API Endpoint: /apis/apps/v1/namespaces/{namespace}/deployments
	Deployment(*runtime_entity.Runtime) connector_deployment_k8s.IConnectorDeployment

	// Metrics connects to the Kubernetes API to retrieve metrics.
	// API Endpoint: /apis/metrics.k8s.io/v1beta1
	Metrics(*runtime_entity.Runtime) connector_metrics_k8s.IConnectorMetrics

	// PersistentVolumeClain PersistentVolumeClaim connects to the Kubernetes API to manage persistent volume claims.
	// API Endpoint: /api/v1/namespaces/{namespace}/persistentvolumeclaims
	PersistentVolumeClain(*runtime_entity.Runtime) connector_pvc_k8s.IConnectorPvc

	// ClusterRole connects to the Kubernetes API to manage cluster roles.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/clusterroles
	ClusterRole(*runtime_entity.Runtime) connector_cluster_role.IConnectorClusterRole

	// ClusterRoleBinding connects to the Kubernetes API to manage cluster role bindings.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/clusterrolebindings
	ClusterRoleBinding(*runtime_entity.Runtime) connector_cluster_role_binding.IConnectorClusterRoleBinding

	// Role connects to the Kubernetes API to manage roles.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/namespaces/{namespace}/roles
	Role(*runtime_entity.Runtime) connector_role.IConnectorRole

	// RoleBinding connects to the Kubernetes API to manage role bindings.
	// API Endpoint: /apis/rbac.authorization.k8s.io/v1/namespaces/{namespace}/rolebindings
	RoleBinding(*runtime_entity.Runtime) connector_role_binding.IConnectorRoleBinding

	// Service connects to the Kubernetes API to manage services.
	// API Endpoint: /api/v1/namespaces/{namespace}/services
	Service(*runtime_entity.Runtime) connector_service.IConnectorService

	// ServiceAccount connects to the Kubernetes API to manage service accounts.
	// API Endpoint: /api/v1/namespaces/{namespace}/serviceaccounts
	ServiceAccount(*runtime_entity.Runtime) connector_service_account.IConnectorServiceAccount

	// StorageClass connects to the Kubernetes API to manage storage classes.
	// API Endpoint: /apis/storage.k8s.io/v1/storageclasses
	StorageClass(*runtime_entity.Runtime) connector_storage_class.IConnectorStorageClass

	// HealthCheck checks the health of the Kubernetes API.
	// API Endpoint: /healthz
	Healthz(*runtime_entity.Runtime) connector_healthz.IConnectorHealthz

	Nodes(*runtime_entity.Runtime) connector_node_k8s.IConnectorNodeK8s
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

func (c *Connector) Namespace(r *runtime_entity.Runtime) connector_namespace_k8s.IConnectorNamespace {
	return connector_namespace_k8s.New(r)
}

func (c *Connector) Pod(r *runtime_entity.Runtime) connector_pod_k8s.IConnectorPod {
	return connector_pod_k8s.New(r)
}

func (c *Connector) Job(r *runtime_entity.Runtime) connector_job_k8s.IConnectorJob {
	return connector_job_k8s.New(r)
}

func (c *Connector) Deployment(r *runtime_entity.Runtime) connector_deployment_k8s.IConnectorDeployment {
	return connector_deployment_k8s.New(r)
}

func (c *Connector) Metrics(r *runtime_entity.Runtime) connector_metrics_k8s.IConnectorMetrics {
	return connector_metrics_k8s.New(r)
}

func (c *Connector) PersistentVolumeClain(r *runtime_entity.Runtime) connector_pvc_k8s.IConnectorPvc {
	return connector_pvc_k8s.New(r)
}

func (c *Connector) ClusterRole(r *runtime_entity.Runtime) connector_cluster_role.IConnectorClusterRole {
	return connector_cluster_role.New(r)
}

func (c *Connector) ClusterRoleBinding(r *runtime_entity.Runtime) connector_cluster_role_binding.IConnectorClusterRoleBinding {
	return connector_cluster_role_binding.New(r)
}

func (c *Connector) Role(r *runtime_entity.Runtime) connector_role.IConnectorRole {
	return connector_role.New(r)
}

func (c *Connector) RoleBinding(r *runtime_entity.Runtime) connector_role_binding.IConnectorRoleBinding {
	return connector_role_binding.New(r)
}

func (c *Connector) Service(r *runtime_entity.Runtime) connector_service.IConnectorService {
	return connector_service.New(r)
}

func (c *Connector) ServiceAccount(r *runtime_entity.Runtime) connector_service_account.IConnectorServiceAccount {
	return connector_service_account.New(r)
}

func (c *Connector) StorageClass(r *runtime_entity.Runtime) connector_storage_class.IConnectorStorageClass {
	return connector_storage_class.New(r)
}

func (c *Connector) Healthz(r *runtime_entity.Runtime) connector_healthz.IConnectorHealthz {
	return connector_healthz.New(r)
}

func (c *Connector) Nodes(r *runtime_entity.Runtime) connector_node_k8s.IConnectorNodeK8s {
	return connector_node_k8s.New(r)
}
