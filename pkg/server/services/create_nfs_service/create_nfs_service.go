package create_nfs_service

import (
	"fmt"
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/entities/nfs_server_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"log"
)

type CreateNfsService struct {
	workflow workflow_entity.Workflow

	connector connector.IConnector
	namespace string
}

const PREFIX_NFS_PROVISIONER = "nfs-provisioner-"

func New() CreateNfsService {
	return CreateNfsService{
		connector: config.App().Connector.K8sConnector,
		namespace: config.App().DefaultNamespace,
	}
}

func (c *CreateNfsService) SetWorkflow(workflow workflow_entity.Workflow) *CreateNfsService {
	c.workflow = workflow
	return c
}

func (c *CreateNfsService) SetNamespace(namespace string) *CreateNfsService {
	c.namespace = namespace
	return c

}

func (c *CreateNfsService) GetWorkflow() workflow_entity.Workflow {
	return c.workflow
}

func (c *CreateNfsService) GetWorkflowIdString() string {
	return fmt.Sprint(c.workflow.GetId())
}

func (c *CreateNfsService) GetNamespace() string {
	return c.namespace
}

func (c *CreateNfsService) createServiceAccount() nfs_server_entity.ServiceAccount {
	serviceAccount := nfs_server_entity.ServiceAccount{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString() + "-service-account",
		},
	}

	return serviceAccount
}

func (c *CreateNfsService) createService() nfs_server_entity.Service {
	service := nfs_server_entity.Service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString(),
			Labels: map[string]string{
				"app": PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString(),
			},
		},
		Spec: nfs_server_entity.ServiceSpec{
			Ports: []nfs_server_entity.ServicePort{
				{Name: "nfs", Port: 2049},
				{Name: "nfs-udp", Port: 2049, Protocol: "UDP"},
				{Name: "nlockmgr", Port: 32803},
				{Name: "nlockmgr-udp", Port: 32803, Protocol: "UDP"},
				{Name: "mountd", Port: 20048},
				{Name: "mountd-udp", Port: 20048, Protocol: "UDP"},
				{Name: "rquotad", Port: 875},
				{Name: "rquotad-udp", Port: 875, Protocol: "UDP"},
				{Name: "rpcbind", Port: 111},
				{Name: "rpcbind-udp", Port: 111, Protocol: "UDP"},
				{Name: "statd", Port: 662},
				{Name: "statd-udp", Port: 662, Protocol: "UDP"},
			},
			Selector: map[string]string{
				"app": PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString(),
			},
		},
	}

	return service
}

func (c *CreateNfsService) createPersistentVolumeClaim() nfs_server_entity.PersistentVolumeClaim {
	pvc := nfs_server_entity.PersistentVolumeClaim{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      c.GetWorkflow().MakeWorkflowPersistentVolumeClaimName(),
		},
		Spec: nfs_server_entity.PersistentVolumeClaimSpec{
			AccessModes: []string{"ReadWriteOnce"},
			Resources: nfs_server_entity.Resources{
				Requests: nfs_server_entity.ResourceRequests{
					Storage: c.GetWorkflow().GetStorageSize(),
				},
			},
			StorageClassName: c.GetWorkflow().GetStorageClassName(),
		},
	}

	return pvc
}

func makeNfsProvisionerName(workflowId int) string {
	return PREFIX_NFS_PROVISIONER + fmt.Sprint(workflowId)
}

func (c *CreateNfsService) createDeployment() nfs_server_entity.Deployment {
	deployment := nfs_server_entity.Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      makeNfsProvisionerName(c.GetWorkflow().GetId()),
		},
		Spec: nfs_server_entity.DeploymentSpec{
			Selector: nfs_server_entity.DeploymentSelector{
				MatchLabels: map[string]string{
					"app": makeNfsProvisionerName(c.GetWorkflow().GetId()),
				},
			},
			Replicas: 1,
			Strategy: nfs_server_entity.DeploymentStrategy{
				Type: "Recreate",
			},
			Template: nfs_server_entity.PodTemplate{
				Metadata: nfs_server_entity.Metadata{
					Labels: map[string]string{
						"app": makeNfsProvisionerName(c.GetWorkflow().GetId()),
					},
				},
				Spec: nfs_server_entity.PodSpec{
					ServiceAccountName: makeNfsProvisionerName(c.GetWorkflow().GetId()) + "-service-account",
					Containers: []nfs_server_entity.Container{
						{
							Name:  makeNfsProvisionerName(c.GetWorkflow().GetId()) + "-server",
							Image: "registry.k8s.io/sig-storage/nfs-provisioner:v4.0.8",
							Ports: []nfs_server_entity.ContainerPort{
								{Name: "nfs", ContainerPort: 2049},
								{Name: "nfs-udp", ContainerPort: 2049, Protocol: "UDP"},
								{Name: "nlockmgr", ContainerPort: 32803},
								{Name: "nlockmgr-udp", ContainerPort: 32803, Protocol: "UDP"},
								{Name: "mountd", ContainerPort: 20048},
								{Name: "mountd-udp", ContainerPort: 20048, Protocol: "UDP"},
								{Name: "rquotad", ContainerPort: 875},
								{Name: "rquotad-udp", ContainerPort: 875, Protocol: "UDP"},
								{Name: "rpcbind", ContainerPort: 111},
								{Name: "rpcbind-udp", ContainerPort: 111, Protocol: "UDP"},
								{Name: "statd", ContainerPort: 662},
								{Name: "statd-udp", ContainerPort: 662, Protocol: "UDP"},
							},
							SecurityContext: nfs_server_entity.SecurityContext{
								Capabilities: nfs_server_entity.Capabilities{
									Add: []string{"DAC_READ_SEARCH", "SYS_RESOURCE"},
								},
							},
							Args: []string{"-provisioner=akoflow.com/nfs-" + c.GetWorkflowIdString()},
							Env: []nfs_server_entity.EnvVar{
								{
									Name: "POD_IP",
									ValueFrom: &nfs_server_entity.EnvVarSource{
										FieldRef: &nfs_server_entity.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{
									Name:  "SERVICE_NAME",
									Value: PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString(),
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &nfs_server_entity.EnvVarSource{
										FieldRef: &nfs_server_entity.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							ImagePullPolicy: "IfNotPresent",
							VolumeMounts: []nfs_server_entity.VolumeMount{
								{
									Name:      c.GetWorkflow().MakeWorkflowPersistentVolumeClaimName(),
									MountPath: c.GetWorkflow().GetMountPath(),
								},
							},
						},
					},
					Volumes: []nfs_server_entity.Volume{
						{
							Name: c.GetWorkflow().MakeWorkflowPersistentVolumeClaimName(),
						},
					},
				},
			},
		},
	}

	return deployment
}

func (c *CreateNfsService) createClusterRole() nfs_server_entity.ClusterRole {
	clusterRole := nfs_server_entity.ClusterRole{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      "nfs-provisioner-runner-" + c.GetWorkflowIdString(),
		},
		Rules: []nfs_server_entity.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list", "watch", "create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "update", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services", "endpoints"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups:     []string{"extensions"},
				Resources:     []string{"podsecuritypolicies"},
				ResourceNames: []string{PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString()},
				Verbs:         []string{"use"},
			},
		},
	}

	return clusterRole
}

func (c *CreateNfsService) createClusterRoleBinding() nfs_server_entity.ClusterRoleBinding {
	clusterRoleBinding := nfs_server_entity.ClusterRoleBinding{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      "run-nfs-provisioner-" + c.GetWorkflowIdString(),
		},
		Subjects: []nfs_server_entity.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString() + "-service-account",
				Namespace: c.GetNamespace(),
			},
		},
		RoleRef: nfs_server_entity.RoleRef{
			Kind:     "ClusterRole",
			Name:     "nfs-provisioner-runner-" + c.GetWorkflowIdString(),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return clusterRoleBinding
}

func (c *CreateNfsService) createRole() nfs_server_entity.Role {
	role := nfs_server_entity.Role{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      "leader-locking-nfs-provisioner-" + c.GetWorkflowIdString(),
		},
		Rules: []nfs_server_entity.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
			},
		},
	}

	return role
}

func (c *CreateNfsService) createRoleBinding() nfs_server_entity.RoleBinding {
	roleBinding := nfs_server_entity.RoleBinding{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      "leader-locking-nfs-provisioner-" + c.GetWorkflowIdString(),
		},
		Subjects: []nfs_server_entity.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      PREFIX_NFS_PROVISIONER + c.GetWorkflowIdString() + "-service-account",
				Namespace: c.GetNamespace(),
			},
		},
		RoleRef: nfs_server_entity.RoleRef{
			Kind:     "Role",
			Name:     "leader-locking-nfs-provisioner-" + c.GetWorkflowIdString(),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return roleBinding
}

func (c *CreateNfsService) createStorageClass() nfs_server_entity.StorageClass {
	storageClass := nfs_server_entity.StorageClass{
		APIVersion: "storage.k8s.io/v1",
		Kind:       "StorageClass",
		Metadata: nfs_server_entity.Metadata{
			Namespace: c.GetNamespace(),
			Name:      c.GetWorkflow().MakeStorageClassNameDistributed(),
		},
		Provisioner:  "akoflow.com/nfs-" + c.GetWorkflowIdString(),
		MountOptions: []string{"vers=4.1"},
	}

	return storageClass
}

func (c *CreateNfsService) Create() bool {
	// Initialize connector
	conn := c.connector

	// ServiceAccount
	serviceAccount := c.createServiceAccount()
	resultServiceAccount := conn.ServiceAccount().CreateServiceAccount(serviceAccount)
	if !resultServiceAccount.Success {
		log.Printf("Failed to create ServiceAccount: %s", resultServiceAccount.Message)
		return false

	}
	fmt.Println(resultServiceAccount.Message)

	// Service
	service := c.createService()
	resultService := conn.Service().CreateService(service)
	if !resultService.Success {
		log.Printf("Failed to create Service: %s", resultService.Message)
		return false
	}
	fmt.Println(resultService.Message)

	// PersistentVolumeClaim
	pvc := c.createPersistentVolumeClaim()
	resultPvc := conn.PersistentVolumeClain().CreatePvc(pvc)
	if !resultPvc.Success {
		log.Printf("Failed to create PersistentVolumeClaim: %s", resultPvc.Message)
		return false

	}
	fmt.Println(resultPvc.Message)

	// Deployment
	deployment := c.createDeployment()
	resultDeployment := conn.Deployment().CreateDeployment(deployment)
	if !resultDeployment.Success {
		log.Printf("Failed to create Deployment: %s", resultDeployment.Message)
		return false

	}
	fmt.Println(resultDeployment.Message)

	// ClusterRole
	clusterRole := c.createClusterRole()
	resultClusterRole := conn.ClusterRole().CreateClusterRole(clusterRole)
	if !resultClusterRole.Success {
		log.Printf("Failed to create ClusterRole: %s", resultClusterRole.Message)
		return false

	}
	fmt.Println(resultClusterRole.Message)

	// ClusterRoleBinding
	clusterRoleBinding := c.createClusterRoleBinding()
	resultClusterRoleBinding := conn.ClusterRoleBinding().CreateClusterRoleBinding(clusterRoleBinding)
	if !resultClusterRoleBinding.Success {
		log.Fatalf("Failed to create ClusterRoleBinding: %s", resultClusterRoleBinding.Message)
	}
	fmt.Println(resultClusterRoleBinding.Message)

	// Role
	role := c.createRole()
	resultRole := conn.Role().CreateRole(role)
	if !resultRole.Success {
		log.Printf("Failed to create Role: %s", resultRole.Message)
		return false

	}
	fmt.Println(resultRole.Message)

	// RoleBinding
	roleBinding := c.createRoleBinding()
	resultRoleBinding := conn.RoleBinding().CreateRoleBinding(roleBinding)
	if !resultRoleBinding.Success {
		log.Printf("Failed to create RoleBinding: %s", resultRoleBinding.Message)
		return false

	}
	fmt.Println(resultRoleBinding.Message)

	// StorageClass
	storageClass := c.createStorageClass()
	resultStorageClass := conn.StorageClass().CreateStorageClass(storageClass)
	if !resultStorageClass.Success {
		log.Printf("Failed to create StorageClass: %s", resultStorageClass.Message)
		return false
	}
	fmt.Println(resultStorageClass.Message)

	return true

}

func (c *CreateNfsService) NfsServerIsCreated() bool {
	conn := config.App().Connector.K8sConnector

	deploymentName := makeNfsProvisionerName(c.GetWorkflow().GetId())
	deployments := conn.Deployment().GetDeployment(c.GetNamespace(), deploymentName)

	if !deployments.Success {
		log.Printf("Failed to get Deployment: %s", deployments.Message)
		return false
	}

	if deployments.Data == nil {
		return false
	}

	return true

}