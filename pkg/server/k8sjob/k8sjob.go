package k8sjob

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
	Template K8sJobTemplate `json:"template"`
}

type K8sJobTemplate struct {
	Spec K8sJobSpecTemplate `json:"spec"`
}

type K8sJobSpecTemplate struct {
	Containers    []K8sJobContainer `json:"containers"`
	RestartPolicy string            `json:"restartPolicy"`
	BackoffLimit  int               `json:"backoffLimit"`
	Volumes       []K8sJobVolume    `json:"volumes"`
}

type K8sJobContainer struct {
	Name         string              `json:"name"`
	Image        string              `json:"image"`
	Command      []string            `json:"command"`
	VolumeMounts []K8sJobVolumeMount `json:"volumeMounts"`
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

// docker run --rm alpine:latest bin/sh -c 'echo ZWNobyAiSGVsbG8gV29ybGQiCnNsZWVwIDUKZWNobyAiSGVsbG8gV29ybGQgQWdhaW4iCnNsZWVwIDUKZWNobyAiSGVsbG8gV29ybGQgT25lIE1vcmUgVGltZSI=| base64 -d| sh'
