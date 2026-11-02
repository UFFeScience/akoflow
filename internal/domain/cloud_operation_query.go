package domain

type CloudOperationFilter struct {
	EnvironmentID    string
	InstanceID       string
	CapacityTargetID string
	ExecutionRunID   string
	Status           string
	Offset           int
	Limit            int
}
