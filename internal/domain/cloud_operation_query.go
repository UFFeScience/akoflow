package domain

type CloudOperationFilter struct {
	EnvironmentID    string
	InstanceID       string
	CapacityTargetID string
	ExecutionRunID   string
	Limit            int
}
