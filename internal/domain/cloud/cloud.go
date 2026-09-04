package cloud

import (
	"time"
)

const DefaultConfigurationID = "akoflow-scientific-worker"

type Compatibility struct {
	Providers        []string `json:"providers,omitempty" yaml:"providers,omitempty"`
	OperatingSystems []string `json:"operatingSystems,omitempty" yaml:"operatingSystems,omitempty"`
	Architectures    []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`
}

type ValidationCheck struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	Capability string `json:"capability,omitempty"`
}

type MachineConfiguration struct {
	ID          string                        `json:"id"`
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	Ownership   string                        `json:"ownership"`
	Enabled     bool                          `json:"enabled"`
	Versions    []MachineConfigurationVersion `json:"versions,omitempty"`
	CreatedAt   time.Time                     `json:"createdAt"`
	UpdatedAt   time.Time                     `json:"updatedAt"`
}

type MachineConfigurationVersion struct {
	ID                     string            `json:"id"`
	MachineConfigurationID string            `json:"machineConfigurationId"`
	Version                int               `json:"version"`
	Status                 string            `json:"status"`
	PlaybookYAML           string            `json:"playbookYaml"`
	ContentSHA256          string            `json:"contentSha256"`
	Compatibility          Compatibility     `json:"compatibility"`
	VariablesSchema        map[string]any    `json:"variablesSchema,omitempty"`
	ValidationChecks       []ValidationCheck `json:"validationChecks,omitempty"`
	CreatedAt              time.Time         `json:"createdAt"`
}

type TargetConfiguration struct {
	ConfigurationVersionID string         `json:"configurationVersionId"`
	ExecutionOrder         int            `json:"executionOrder"`
	Variables              map[string]any `json:"variables,omitempty"`
	Required               bool           `json:"required"`
	Enabled                bool           `json:"enabled"`
}

type CapacityTarget struct {
	ID                    string                `json:"id"`
	EnvironmentID         string                `json:"environmentId"`
	Name                  string                `json:"name"`
	Provider              string                `json:"provider"`
	ProviderMachineType   string                `json:"providerMachineType"`
	Region                string                `json:"region"`
	ZonePolicy            string                `json:"zonePolicy"`
	FixedZone             string                `json:"fixedZone,omitempty"`
	ImageReference        string                `json:"imageReference"`
	Architecture          string                `json:"architecture"`
	VCPU                  int                   `json:"vcpu"`
	MemoryMiB             int64                 `json:"memoryMiB"`
	ProvisioningMode      string                `json:"provisioningMode"`
	MaximumInstances      int                   `json:"maximumInstances"`
	LifecyclePolicy       string                `json:"lifecyclePolicy"`
	Configuration         map[string]any        `json:"configuration,omitempty"`
	MachineConfigurations []TargetConfiguration `json:"machineConfigurations,omitempty"`
	Enabled               bool                  `json:"enabled"`
	CreatedAt             time.Time             `json:"createdAt"`
}

type PlaybookValidation struct {
	Valid  bool     `json:"valid"`
	SHA256 string   `json:"sha256,omitempty"`
	Errors []string `json:"errors,omitempty"`
}

type MachineOffering struct {
	Provider         string         `json:"provider"`
	ProviderTypeID   string         `json:"providerTypeId"`
	Region           string         `json:"region"`
	Zones            []string       `json:"zones"`
	Family           string         `json:"family,omitempty"`
	VCPU             int            `json:"vcpu"`
	MemoryMiB        int64          `json:"memoryMiB"`
	Architecture     string         `json:"architecture"`
	GPUCount         int            `json:"gpuCount"`
	GPUModel         string         `json:"gpuModel,omitempty"`
	SupportsSpot     bool           `json:"supportsSpot"`
	Available        bool           `json:"available"`
	ProviderMetadata map[string]any `json:"providerMetadata,omitempty"`
}

type ImageOffering struct {
	Provider         string         `json:"provider"`
	ProviderImageID  string         `json:"providerImageId"`
	Name             string         `json:"name"`
	Publisher        string         `json:"publisher,omitempty"`
	Family           string         `json:"family,omitempty"`
	OperatingSystem  string         `json:"operatingSystem"`
	Architecture     string         `json:"architecture"`
	Source           string         `json:"source"`
	Status           string         `json:"status"`
	MinimumDiskGiB   int64          `json:"minimumDiskGiB"`
	Deprecated       bool           `json:"deprecated"`
	ProviderMetadata map[string]any `json:"providerMetadata,omitempty"`
}

type DiskOffering struct {
	Provider         string         `json:"provider"`
	ProviderTypeID   string         `json:"providerTypeId"`
	Name             string         `json:"name"`
	Region           string         `json:"region"`
	Zones            []string       `json:"zones"`
	Available        bool           `json:"available"`
	ProviderMetadata map[string]any `json:"providerMetadata,omitempty"`
}

type Catalog struct {
	Provider      string            `json:"provider"`
	EnvironmentID string            `json:"environmentId"`
	Project       string            `json:"project,omitempty"`
	Region        string            `json:"region,omitempty"`
	Source        string            `json:"source"`
	Machines      []MachineOffering `json:"machines"`
	Images        []ImageOffering   `json:"images"`
	Disks         []DiskOffering    `json:"disks"`
	Warnings      []string          `json:"warnings,omitempty"`
	DiscoveredAt  time.Time         `json:"discoveredAt"`
}
