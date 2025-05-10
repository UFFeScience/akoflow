package types_api

type ApiWorkflowActivityType struct {
	Id            int      `json:"id"`
	WorkflowId    int      `json:"workflowId"`
	Status        int      `json:"status"`
	Runtime       string   `yaml:"runtime" json:"runtime"`
	Name          string   `yaml:"name" json:"name"`
	Run           string   `yaml:"run" json:"run"`
	MemoryLimit   string   `yaml:"memoryLimit" json:"memoryLimit"`
	CpuLimit      string   `yaml:"cpuLimit" json:"cpuLimit"`
	DependsOn     []string `yaml:"dependsOn" json:"dependsOn"`
	NodeSelector  string   `yaml:"nodeSelector" json:"nodeSelector"`
	KeepDisk      bool     `yaml:"keepDisk" json:"keepDisk"`
	CreatedAt     string   `yaml:"createdAt" json:"createdAt"`
	StartedAt     string   `yaml:"startedAt" json:"startedAt"`
	FinishedAt    string   `yaml:"finishedAt" json:"finishedAt"`
	ExecutionTime string   `yaml:"executionTime" json:"executionTime"`
}
