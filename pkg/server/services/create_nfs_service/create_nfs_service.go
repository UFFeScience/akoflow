package create_nfs_service

import (
	"fmt"
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/entities/nfs_server_entity"
	"log"
)

type CreateNfsService struct {
}

func New() CreateNfsService {
	return CreateNfsService{}
}

func (c *CreateNfsService) CreateServiceAccount() nfs_server_entity.ServiceAccount {
	serviceAccount := nfs_server_entity.ServiceAccount{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "nfs-provisioner",
		},
	}

	return serviceAccount
}

func (c *CreateNfsService) CreateService() nfs_server_entity.Service {
	service := nfs_server_entity.Service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "nfs-provisioner",
			Labels: map[string]string{
				"app": "nfs-provisioner",
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
				"app": "nfs-provisioner",
			},
		},
	}

	return service
}

func (c *CreateNfsService) CreatePersistentVolumeClaim() nfs_server_entity.PersistentVolumeClaim {
	pvc := nfs_server_entity.PersistentVolumeClaim{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "workflow-x-volume",
		},
		Spec: nfs_server_entity.PersistentVolumeClaimSpec{
			AccessModes: []string{"ReadWriteOnce"},
			Resources: nfs_server_entity.Resources{
				Requests: nfs_server_entity.ResourceRequests{
					Storage: "128Mi",
				},
			},
			StorageClassName: "hostpath",
		},
	}

	return pvc
}

func (c *CreateNfsService) CreateDeployment() nfs_server_entity.Deployment {
	deployment := nfs_server_entity.Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "nfs-provisioner",
		},
		Spec: nfs_server_entity.DeploymentSpec{
			Selector: nfs_server_entity.DeploymentSelector{
				MatchLabels: map[string]string{
					"app": "nfs-provisioner",
				},
			},
			Replicas: 1,
			Strategy: nfs_server_entity.DeploymentStrategy{
				Type: "Recreate",
			},
			Template: nfs_server_entity.PodTemplate{
				Metadata: nfs_server_entity.Metadata{
					Labels: map[string]string{
						"app": "nfs-provisioner",
					},
				},
				Spec: nfs_server_entity.PodSpec{
					ServiceAccountName: "nfs-provisioner",
					Containers: []nfs_server_entity.Container{
						{
							Name:  "nfs-provisioner",
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
							Args: []string{"-provisioner=akoflow.ovvesley/nfs"},
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
									Value: "nfs-provisioner",
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
									Name:      "workflow-x-volume",
									MountPath: "/export",
								},
							},
						},
					},
					Volumes: []nfs_server_entity.Volume{
						{
							Name: "workflow-x-volume",
						},
					},
				},
			},
		},
	}

	return deployment
}

func (c *CreateNfsService) CreateClusterRole() nfs_server_entity.ClusterRole {
	clusterRole := nfs_server_entity.ClusterRole{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "nfs-provisioner-runner",
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
				ResourceNames: []string{"nfs-provisioner"},
				Verbs:         []string{"use"},
			},
		},
	}

	return clusterRole
}

func (c *CreateNfsService) CreateClusterRoleBinding() nfs_server_entity.ClusterRoleBinding {
	clusterRoleBinding := nfs_server_entity.ClusterRoleBinding{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "run-nfs-provisioner",
		},
		Subjects: []nfs_server_entity.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "nfs-provisioner",
				Namespace: "akoflow",
			},
		},
		RoleRef: nfs_server_entity.RoleRef{
			Kind:     "ClusterRole",
			Name:     "nfs-provisioner-runner",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return clusterRoleBinding
}

func (c *CreateNfsService) CreateRole() nfs_server_entity.Role {
	role := nfs_server_entity.Role{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "leader-locking-nfs-provisioner",
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

func (c *CreateNfsService) CreateRoleBinding() nfs_server_entity.RoleBinding {
	roleBinding := nfs_server_entity.RoleBinding{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "leader-locking-nfs-provisioner",
		},
		Subjects: []nfs_server_entity.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "nfs-provisioner",
				Namespace: "akoflow",
			},
		},
		RoleRef: nfs_server_entity.RoleRef{
			Kind:     "Role",
			Name:     "leader-locking-nfs-provisioner",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return roleBinding
}

func (c *CreateNfsService) CreateStorageClass() nfs_server_entity.StorageClass {
	storageClass := nfs_server_entity.StorageClass{
		APIVersion: "storage.k8s.io/v1",
		Kind:       "StorageClass",
		Metadata: nfs_server_entity.Metadata{
			Namespace: "akoflow",
			Name:      "akoflow-nfs",
		},
		Provisioner:  "akoflow.ovvesley/nfs",
		MountOptions: []string{"vers=4.1"},
	}

	return storageClass
}

func (c *CreateNfsService) Create() {
	// Initialize connector
	conn := connector.New()

	// ServiceAccount
	serviceAccount := c.CreateServiceAccount()
	resultServiceAccount := conn.ServiceAccount().CreateServiceAccount(serviceAccount)
	//if !resultServiceAccount.Success {
	//	log.Fatalf("Failed to create ServiceAccount: %s", resultServiceAccount.Message)
	//}
	fmt.Println(resultServiceAccount.Message)

	// Service
	service := c.CreateService()
	resultService := conn.Service().CreateService(service)
	//if !resultService.Success {
	//	log.Fatalf("Failed to create Service: %s", resultService.Message)
	//}
	fmt.Println(resultService.Message)

	// PersistentVolumeClaim
	pvc := c.CreatePersistentVolumeClaim()
	resultPvc := conn.PersistentVolumeClain().CreatePvc(pvc)
	//if !resultPvc.Success {
	//	log.Fatalf("Failed to create PersistentVolumeClaim: %s", resultPvc.Message)
	//}
	fmt.Println(resultPvc.Message)

	// Deployment
	deployment := c.CreateDeployment()
	resultDeployment := conn.Deployment().CreateDeployment(deployment)
	//if !resultDeployment.Success {
	//	log.Fatalf("Failed to create Deployment: %s", resultDeployment.Message)
	//}
	fmt.Println(resultDeployment.Message)

	// ClusterRole
	clusterRole := c.CreateClusterRole()
	resultClusterRole := conn.ClusterRole().CreateClusterRole(clusterRole)
	//if !resultClusterRole.Success {
	//	log.Fatalf("Failed to create ClusterRole: %s", resultClusterRole.Message)
	//}
	fmt.Println(resultClusterRole.Message)

	// ClusterRoleBinding
	clusterRoleBinding := c.CreateClusterRoleBinding()
	resultClusterRoleBinding := conn.ClusterRoleBinding().CreateClusterRoleBinding(clusterRoleBinding)
	//if !resultClusterRoleBinding.Success {
	//	log.Fatalf("Failed to create ClusterRoleBinding: %s", resultClusterRoleBinding.Message)
	//}
	fmt.Println(resultClusterRoleBinding.Message)

	// Role
	role := c.CreateRole()
	resultRole := conn.Role().CreateRole(role)
	//if !resultRole.Success {
	//	log.Fatalf("Failed to create Role: %s", resultRole.Message)
	//}
	fmt.Println(resultRole.Message)

	// RoleBinding
	roleBinding := c.CreateRoleBinding()
	resultRoleBinding := conn.RoleBinding().CreateRoleBinding(roleBinding)
	//if !resultRoleBinding.Success {
	//	log.Fatalf("Failed to create RoleBinding: %s", resultRoleBinding.Message)
	//}
	fmt.Println(resultRoleBinding.Message)

	// StorageClass
	storageClass := c.CreateStorageClass()
	resultStorageClass := conn.StorageClass().CreateStorageClass(storageClass)
	if !resultStorageClass.Success {
		log.Fatalf("Failed to create StorageClass: %s", resultStorageClass.Message)
	}
	fmt.Println(resultStorageClass.Message)

}
