package workflow

import (
	"encoding/base64"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name string       `yaml:"name"`
	Spec WorkflowSpec `yaml:"spec"`
}

type WorkflowSpec struct {
	MemoryLimit string               `yaml:"memoryLimit"`
	CPULimit    int                  `yaml:"cpuLimit"`
	Tries       int                  `yaml:"tries"`
	Image       string               `yaml:"image"`
	Activities  []WorkflowActivities `yaml:"activities"`
}

type WorkflowActivities struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
}

func New(workflowBase64 string) Workflow {
	byteWorkflow, _ := base64.StdEncoding.DecodeString(workflowBase64)

	stringWorkflow := string(byteWorkflow)

	yamlWorkflow := Workflow{}
	err := yaml.Unmarshal([]byte(stringWorkflow), &yamlWorkflow)

	if err != nil {
		return Workflow{}
	}

	return yamlWorkflow
}

func (w Workflow) ToBase64() string {
	return ""
}

func (w Workflow) ToYaml() string {
	return ""
}

func (w Workflow) Validate() bool {
	return true
}
