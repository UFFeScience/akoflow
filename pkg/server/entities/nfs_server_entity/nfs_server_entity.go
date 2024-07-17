package nfs_server_entity

// ServiceAccount represents a Kubernetes ServiceAccount
type ServiceAccount struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
}

// Metadata represents metadata for Kubernetes objects
type Metadata struct {
	Namespace string            `yaml:"namespace"`
	Name      string            `yaml:"name"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

// Service represents a Kubernetes Service
type Service struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   Metadata    `yaml:"metadata"`
	Spec       ServiceSpec `yaml:"spec"`
}

// ServiceSpec represents the specification of a Kubernetes Service
type ServiceSpec struct {
	Ports    []ServicePort     `yaml:"ports"`
	Selector map[string]string `yaml:"selector"`
}

// ServicePort represents a port for a Kubernetes Service
type ServicePort struct {
	Name     string `yaml:"name"`
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol,omitempty"`
}

// PersistentVolumeClaim represents a Kubernetes PersistentVolumeClaim
type PersistentVolumeClaim struct {
	APIVersion string                    `yaml:"apiVersion"`
	Kind       string                    `yaml:"kind"`
	Metadata   Metadata                  `yaml:"metadata"`
	Spec       PersistentVolumeClaimSpec `yaml:"spec"`
}

// PersistentVolumeClaimSpec represents the specification of a PersistentVolumeClaim
type PersistentVolumeClaimSpec struct {
	AccessModes      []string  `yaml:"accessModes"`
	Resources        Resources `yaml:"resources"`
	StorageClassName string    `yaml:"storageClassName"`
}

// Resources represents resource requests for a PersistentVolumeClaim
type Resources struct {
	Requests ResourceRequests `yaml:"requests"`
}

// ResourceRequests represents resource requests for a PersistentVolumeClaim
type ResourceRequests struct {
	Storage string `yaml:"storage"`
}

// Deployment represents a Kubernetes Deployment
type Deployment struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   Metadata       `yaml:"metadata"`
	Spec       DeploymentSpec `yaml:"spec"`
}

// DeploymentSpec represents the specification of a Kubernetes Deployment
type DeploymentSpec struct {
	Selector DeploymentSelector `yaml:"selector"`
	Replicas int                `yaml:"replicas"`
	Strategy DeploymentStrategy `yaml:"strategy"`
	Template PodTemplate        `yaml:"template"`
}

// DeploymentSelector represents the selector for a Deployment
type DeploymentSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

// DeploymentStrategy represents the strategy for a Deployment
type DeploymentStrategy struct {
	Type string `yaml:"type"`
}

// PodTemplate represents the template for pods in a Deployment
type PodTemplate struct {
	Metadata Metadata `yaml:"metadata"`
	Spec     PodSpec  `yaml:"spec"`
}

// PodSpec represents the specification for a pod
type PodSpec struct {
	ServiceAccountName string      `yaml:"serviceAccountName"`
	Containers         []Container `yaml:"containers"`
	Volumes            []Volume    `yaml:"volumes"`
}

// Container represents a container in a pod
type Container struct {
	Name            string          `yaml:"name"`
	Image           string          `yaml:"image"`
	Ports           []ContainerPort `yaml:"ports"`
	SecurityContext SecurityContext `yaml:"securityContext"`
	Args            []string        `yaml:"args"`
	Env             []EnvVar        `yaml:"env"`
	ImagePullPolicy string          `yaml:"imagePullPolicy"`
	VolumeMounts    []VolumeMount   `yaml:"volumeMounts"`
}

// ContainerPort represents a port in a container
type ContainerPort struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty"`
}

// SecurityContext represents the security context for a container
type SecurityContext struct {
	Capabilities Capabilities `yaml:"capabilities"`
}

// Capabilities represents capabilities for a security context
type Capabilities struct {
	Add []string `yaml:"add"`
}

// EnvVar represents an environment variable for a container
type EnvVar struct {
	Name      string        `yaml:"name"`
	Value     string        `yaml:"value,omitempty"`
	ValueFrom *EnvVarSource `yaml:"valueFrom,omitempty"`
}

// EnvVarSource represents the source for an environment variable's value
type EnvVarSource struct {
	FieldRef *ObjectFieldSelector `yaml:"fieldRef"`
}

// ObjectFieldSelector selects a field of an object
type ObjectFieldSelector struct {
	FieldPath string `yaml:"fieldPath"`
}

// VolumeMount represents a volume mount for a container
type VolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
}

// Volume represents a volume in a pod
type Volume struct {
	Name string `yaml:"name"`
}

// ClusterRole represents a Kubernetes ClusterRole
type ClusterRole struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   Metadata     `yaml:"metadata"`
	Rules      []PolicyRule `yaml:"rules"`
}

// PolicyRule represents a policy rule for a ClusterRole
type PolicyRule struct {
	APIGroups     []string `yaml:"apiGroups"`
	Resources     []string `yaml:"resources"`
	Verbs         []string `yaml:"verbs"`
	ResourceNames []string `yaml:"resourceNames,omitempty"`
}

// ClusterRoleBinding represents a Kubernetes ClusterRoleBinding
type ClusterRoleBinding struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Metadata   Metadata  `yaml:"metadata"`
	Subjects   []Subject `yaml:"subjects"`
	RoleRef    RoleRef   `yaml:"roleRef"`
}

// Subject represents a subject in a ClusterRoleBinding
type Subject struct {
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// RoleRef represents a role reference in a ClusterRoleBinding
type RoleRef struct {
	Kind     string `yaml:"kind"`
	Name     string `yaml:"name"`
	APIGroup string `yaml:"apiGroup"`
}

// Role represents a Kubernetes Role
type Role struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   Metadata     `yaml:"metadata"`
	Rules      []PolicyRule `yaml:"rules"`
}

// RoleBinding represents a Kubernetes RoleBinding
type RoleBinding struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Metadata   Metadata  `yaml:"metadata"`
	Subjects   []Subject `yaml:"subjects"`
	RoleRef    RoleRef   `yaml:"roleRef"`
}

// StorageClass represents a Kubernetes StorageClass
type StorageClass struct {
	APIVersion   string   `yaml:"apiVersion"`
	Kind         string   `yaml:"kind"`
	Metadata     Metadata `yaml:"metadata"`
	Provisioner  string   `yaml:"provisioner"`
	MountOptions []string `yaml:"mountOptions"`
}
