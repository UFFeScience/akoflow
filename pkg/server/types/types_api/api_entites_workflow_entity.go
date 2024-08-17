package types_api

type ApiWorkflowType struct {
	Name   string              `yaml:"name" json:"name"`
	Status int                 `yaml:"status" json:"status"`
	Id     int                 `json:"id"`
	Spec   ApiWorkflowSpecType `yaml:"spec" json:"spec"`
}

type ApiWorkflowSpecType struct {
	Image            string                           `yaml:"image" json:"image"`
	Namespace        string                           `yaml:"namespace" json:"namespace"`
	StorageClassName string                           `yaml:"storageClassName" json:"storageClassName"`
	StorageSize      string                           `yaml:"storageSize" json:"storageSize"`
	StoragePolicy    ApiWorkflowSpecStoragePolicyType `yaml:"storagePolicy" json:"storagePolicy"`
	MountPath        string                           `yaml:"mountPath" json:"mountPath"`
	Activities       []ApiWorkflowActivityType        `yaml:"activities" json:"activities"`
	CreatedAt        string                           `yaml:"createdAt" json:"createdAt"`
	StartExecution   string                           `yaml:"startExecution" json:"startExecution"`
	EndExecution     string                           `yaml:"endExecution" json:"endExecution"`
	ExecutionTime    string                           `yaml:"executionTime" json:"executionTime"`
	LongestActivity  string                           `yaml:"longestActivity" json:"longestActivity"`
	DiskUsage        string                           `yaml:"diskUsage" json:"diskUsage"`
}

type ApiWorkflowSpecStoragePolicyType struct {
	Type string `yaml:"type" json:"type"` // "distributed" or "standalone"
}
