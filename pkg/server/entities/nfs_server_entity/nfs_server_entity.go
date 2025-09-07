package nfs_server_entity

// ServiceAccount represents a Kubernetes ServiceAccount
type ServiceAccount struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
}

// Metadata represents metadata for Kubernetes objects
type Metadata struct {
	Namespace string            `yaml:"namespace" json:"namespace"`
	Name      string            `yaml:"name" json:"name"`
	Labels    map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// Service represents a Kubernetes Service
type Service struct {
	APIVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Metadata   Metadata    `yaml:"metadata" json:"metadata"`
	Spec       ServiceSpec `yaml:"spec" json:"spec"`
}

// ServiceSpec represents the specification of a Kubernetes Service
type ServiceSpec struct {
	Ports    []ServicePort     `yaml:"ports" json:"ports"`
	Selector map[string]string `yaml:"selector" json:"selector"`
}

// ServicePort represents a port for a Kubernetes Service
type ServicePort struct {
	Name     string `yaml:"name" json:"name"`
	Port     int    `yaml:"port" json:"port"`
	Protocol string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

// PersistentVolumeClaim represents a Kubernetes PersistentVolumeClaim
type PersistentVolumeClaim struct {
	APIVersion string                    `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                    `yaml:"kind" json:"kind"`
	Metadata   Metadata                  `yaml:"metadata" json:"metadata"`
	Spec       PersistentVolumeClaimSpec `yaml:"spec" json:"spec"`
}

// PersistentVolumeClaimSpec represents the specification of a PersistentVolumeClaim
type PersistentVolumeClaimSpec struct {
	AccessModes      []string  `yaml:"accessModes" json:"accessModes"`
	Resources        Resources `yaml:"resources" json:"resources"`
	StorageClassName string    `yaml:"storageClassName" json:"storageClassName"`
}

// Resources represents resource requests for a PersistentVolumeClaim
type Resources struct {
	Requests ResourceRequests `yaml:"requests" json:"requests"`
}

// ResourceRequests represents resource requests for a PersistentVolumeClaim
type ResourceRequests struct {
	Storage string `yaml:"storage" json:"storage"`
}

// Deployment represents a Kubernetes Deployment
type Deployment struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       string         `yaml:"kind" json:"kind"`
	Metadata   Metadata       `yaml:"metadata" json:"metadata"`
	Spec       DeploymentSpec `yaml:"spec" json:"spec"`
}

// DeploymentSpec represents the specification of a Kubernetes Deployment
type DeploymentSpec struct {
	Selector DeploymentSelector `yaml:"selector" json:"selector"`
	Replicas int                `yaml:"replicas" json:"replicas"`
	Strategy DeploymentStrategy `yaml:"strategy" json:"strategy"`
	Template PodTemplate        `yaml:"template" json:"template"`
}

// DeploymentSelector represents the selector for a Deployment
type DeploymentSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels" json:"matchLabels"`
}

// DeploymentStrategy represents the strategy for a Deployment
type DeploymentStrategy struct {
	Type string `yaml:"type" json:"type"`
}

// PodTemplate represents the template for pods in a Deployment
type PodTemplate struct {
	Metadata Metadata `yaml:"metadata" json:"metadata"`
	Spec     PodSpec  `yaml:"spec" json:"spec"`
}

// PodSpec represents the specification for a pod
type PodSpec struct {
	ServiceAccountName string      `yaml:"serviceAccountName" json:"serviceAccountName"`
	Containers         []Container `yaml:"containers" json:"containers"`
	Volumes            []Volume    `yaml:"volumes" json:"volumes"`
}

// Container represents a container in a pod
type Container struct {
	Name            string          `yaml:"name" json:"name"`
	Image           string          `yaml:"image" json:"image"`
	Ports           []ContainerPort `yaml:"ports" json:"ports"`
	SecurityContext SecurityContext `yaml:"securityContext" json:"securityContext"`
	Args            []string        `yaml:"args" json:"args"`
	Env             []EnvVar        `yaml:"env" json:"env"`
	ImagePullPolicy string          `yaml:"imagePullPolicy" json:"imagePullPolicy"`
	VolumeMounts    []VolumeMount   `yaml:"volumeMounts" json:"volumeMounts"`
}

// ContainerPort represents a port in a container
type ContainerPort struct {
	Name          string `yaml:"name" json:"name"`
	ContainerPort int    `yaml:"containerPort" json:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

// SecurityContext represents the security context for a container
type SecurityContext struct {
	Capabilities Capabilities `yaml:"capabilities" json:"capabilities"`
}

// Capabilities represents capabilities for a security context
type Capabilities struct {
	Add []string `yaml:"add" json:"add"`
}

// EnvVar represents an environment variable for a container
type EnvVar struct {
	Name      string        `yaml:"name" json:"name"`
	Value     string        `yaml:"value,omitempty" json:"value,omitempty"`
	ValueFrom *EnvVarSource `yaml:"valueFrom,omitempty" json:"valueFrom,omitempty"`
}

// EnvVarSource represents the source for an environment variable's value
type EnvVarSource struct {
	FieldRef *ObjectFieldSelector `yaml:"fieldRef" json:"fieldRef"`
}

// ObjectFieldSelector selects a field of an object
type ObjectFieldSelector struct {
	FieldPath string `yaml:"fieldPath" json:"fieldPath"`
}

// VolumeMount represents a volume mount for a container
type VolumeMount struct {
	Name      string `yaml:"name" json:"name"`
	MountPath string `yaml:"mountPath" json:"mountPath"`
}

// Volume represents a volume in a pod
type Volume struct {
	Name string `yaml:"name" json:"name"`
}

// ClusterRole represents a Kubernetes ClusterRole
type ClusterRole struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"`
	Metadata   Metadata     `yaml:"metadata" json:"metadata"`
	Rules      []PolicyRule `yaml:"rules" json:"rules"`
}

// PolicyRule represents a policy rule for a ClusterRole
type PolicyRule struct {
	APIGroups     []string `yaml:"apiGroups" json:"apiGroups"`
	Resources     []string `yaml:"resources" json:"resources"`
	Verbs         []string `yaml:"verbs" json:"verbs"`
	ResourceNames []string `yaml:"resourceNames,omitempty" json:"resourceNames,omitempty"`
}

// ClusterRoleBinding represents a Kubernetes ClusterRoleBinding
type ClusterRoleBinding struct {
	APIVersion string    `yaml:"apiVersion" json:"apiVersion"`
	Kind       string    `yaml:"kind" json:"kind"`
	Metadata   Metadata  `yaml:"metadata" json:"metadata"`
	Subjects   []Subject `yaml:"subjects" json:"subjects"`
	RoleRef    RoleRef   `yaml:"roleRef" json:"roleRef"`
}

// Subject represents a subject in a ClusterRoleBinding
type Subject struct {
	Kind      string `yaml:"kind" json:"kind"`
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace" json:"namespace"`
}

// RoleRef represents a role reference in a ClusterRoleBinding
type RoleRef struct {
	Kind     string `yaml:"kind" json:"kind"`
	Name     string `yaml:"name" json:"name"`
	APIGroup string `yaml:"apiGroup" json:"apiGroup"`
}

// Role represents a Kubernetes Role
type Role struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"`
	Metadata   Metadata     `yaml:"metadata" json:"metadata"`
	Rules      []PolicyRule `yaml:"rules" json:"rules"`
}

// RoleBinding represents a Kubernetes RoleBinding
type RoleBinding struct {
	APIVersion string    `yaml:"apiVersion" json:"apiVersion"`
	Kind       string    `yaml:"kind" json:"kind"`
	Metadata   Metadata  `yaml:"metadata" json:"metadata"`
	Subjects   []Subject `yaml:"subjects" json:"subjects"`
	RoleRef    RoleRef   `yaml:"roleRef" json:"roleRef"`
}

// StorageClass represents a Kubernetes StorageClass
type StorageClass struct {
	APIVersion   string   `yaml:"apiVersion" json:"apiVersion"`
	Kind         string   `yaml:"kind" json:"kind"`
	Metadata     Metadata `yaml:"metadata" json:"metadata"`
	Provisioner  string   `yaml:"provisioner" json:"provisioner"`
	MountOptions []string `yaml:"mountOptions" json:"mountOptions"`
	// VolumeBindingMode is a pointer to a string to allow for nil values
	VolumeBindingMode string `yaml:"volumeBindingMode,omitempty" json:"volumeBindingMode,omitempty"`
}
