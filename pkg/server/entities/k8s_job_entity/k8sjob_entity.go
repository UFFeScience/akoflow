package k8s_job_entity

import (
	"encoding/base64"
	"gopkg.in/yaml.v3"
)

type K8sJob struct {
	ApiVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   K8sJobMetadata `json:"metadata"`
	Spec       K8sJobSpec     `json:"spec"`
}

type K8sJobMetadata struct {
	Name string `json:"name"`
}

type K8sJobSpec struct {
	Template     K8sJobTemplate `json:"template"`
	BackoffLimit int            `json:"backoffLimit"`
}

type K8sJobTemplate struct {
	Spec K8sJobSpecTemplate `json:"spec"`
}

type K8sJobSpecTemplate struct {
	Containers    []K8sJobContainer `json:"containers"`
	RestartPolicy string            `json:"restartPolicy"`
	BackoffLimit  int               `json:"backoffLimit"`
	Volumes       []K8sJobVolume    `json:"volumes"`
	NodeSelector  map[string]string `json:"nodeSelector"`
}

type K8sJobContainer struct {
	Name         string              `json:"name"`
	Image        string              `json:"image"`
	Command      []string            `json:"command"`
	VolumeMounts []K8sJobVolumeMount `json:"volumeMounts"`
	Resources    K8sJobResources     `json:"resources"`
	Env          []K8sJobEnv         `json:"env"`
}

type K8sJobEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type K8sJobResources struct {
	Limits K8sJobResourcesLimits `json:"limits"`
}

type K8sJobResourcesLimits struct {
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`
}

type K8sJobVolume struct {
	Name                  string `json:"name"`
	PersistentVolumeClaim struct {
		ClaimName string `json:"claimName"`
	} `json:"persistentVolumeClaim"`
}

type K8sJobVolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

func (k *K8sJob) ToYaml() string {
	workflowStringByte, _ := yaml.Marshal(k)
	workflowStringYaml := string(workflowStringByte)
	return workflowStringYaml
}

func (k *K8sJob) GetBase64Jobs() string {
	y, _ := yaml.Marshal(k)
	return base64.StdEncoding.EncodeToString(y)
}
