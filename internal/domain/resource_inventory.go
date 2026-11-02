package domain

type ResourceInventoryGroup struct {
	EnvironmentDefinition
	CloudCapacityTargets []CloudCapacityTarget      `json:"cloudCapacityTargets"`
	CloudInstances       []CloudProvisionedInstance `json:"cloudInstances"`
}

type CloudInventory struct {
	Targets   []CloudCapacityTarget      `json:"targets"`
	Instances []CloudProvisionedInstance `json:"instances"`
}
