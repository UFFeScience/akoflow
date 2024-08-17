package types_api

type ApiWorkflowActivityType struct {
	Id           int      `json:"id"`
	WorkflowId   int      `json:"workflowId"`
	Status       int      `json:"status"`
	Name         string   `yaml:"name" json:"name"`
	Run          string   `yaml:"run" json:"run"`
	MemoryLimit  string   `yaml:"memoryLimit" json:"memoryLimit"`
	CpuLimit     string   `yaml:"cpuLimit" json:"cpuLimit"`
	DependsOn    []string `yaml:"dependsOn" json:"dependsOn"`
	NodeSelector string   `yaml:"nodeSelector" json:"nodeSelector"`
	KeepDisk     bool     `yaml:"keepDisk" json:"keepDisk"`
}
