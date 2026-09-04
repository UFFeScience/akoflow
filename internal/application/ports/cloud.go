package ports

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type CloudConfigurationStore interface {
	EnsureDefaults(context.Context) error
	CreateMachineConfiguration(context.Context, domain.MachineConfiguration) error
	CreateMachineConfigurationVersion(context.Context, domain.MachineConfigurationVersion) error
	ListMachineConfigurations(context.Context) ([]domain.MachineConfiguration, error)
	FindMachineConfiguration(context.Context, string) (*domain.MachineConfiguration, error)
	FindMachineConfigurationVersion(context.Context, string) (*domain.MachineConfigurationVersion, error)
	CreateCapacityTarget(context.Context, domain.CloudCapacityTarget) error
	ListCapacityTargets(context.Context, string) ([]domain.CloudCapacityTarget, error)
	FindCapacityTarget(context.Context, string) (*domain.CloudCapacityTarget, error)
	CreateProvisionedInstance(context.Context, domain.CloudProvisionedInstance) error
	UpdateProvisionedInstance(context.Context, domain.CloudProvisionedInstance) error
	FindProvisionedInstance(context.Context, string) (*domain.CloudProvisionedInstance, error)
	ListProvisionedInstances(context.Context, string) ([]domain.CloudProvisionedInstance, error)
}

type CloudCredentialResolver interface {
	Resolve(string) ([]byte, error)
}

type CloudCatalogProvider interface {
	Provider() string
	Discover(context.Context, domain.EnvironmentConnection, []byte) (domain.CloudCatalog, error)
}

type CloudCredentialValidator interface {
	ValidateCredential(context.Context, domain.EnvironmentConnection, []byte) error
}

type CloudCatalog interface {
	Discover(context.Context, string) (domain.CloudCatalog, error)
	Cached(context.Context, string) (*domain.CloudCatalog, error)
	Validate(context.Context, domain.EnvironmentConnection, []byte) (domain.CloudCatalog, error)
}

type CloudCatalogStore interface {
	SaveCloudCatalog(context.Context, domain.CloudCatalog) error
	FindCloudCatalog(context.Context, string) (*domain.CloudCatalog, error)
}

type CloudProvisioner interface {
	Provision(context.Context, string, domain.CloudProvisionRequest) (domain.CloudProvisionedInstance, error)
	Destroy(context.Context, string) (domain.CloudProvisionedInstance, error)
	Release(context.Context, []string) error
}

type TerraformProvisionSpec struct {
	InstanceID string
	Target     domain.CloudCapacityTarget
	Credential []byte
	PublicKey  string
	SSHUser    string
}

type TerraformResult struct {
	ProviderID     string
	PublicAddress  string
	PrivateAddress string
	Disk           map[string]any
	Output         map[string]any
}

type TerraformRunner interface {
	Apply(context.Context, TerraformProvisionSpec) (TerraformResult, error)
	Start(context.Context, string) (TerraformResult, error)
	Destroy(context.Context, string) error
	Stop(context.Context, string) error
}

type CloudSSHKey struct {
	CredentialRef string
	PublicKey     string
}

type CloudSSHKeyManager interface {
	Ensure(string, string) (CloudSSHKey, error)
}

type MachineConfigurationSpec struct {
	InstanceID    string
	Address       string
	SSHUser       string
	CredentialRef string
	PlaybookYAML  string
	Variables     map[string]any
	Checks        []domain.MachineConfigurationValidationCheck
}

type MachineConfigurator interface {
	Configure(context.Context, MachineConfigurationSpec) error
}
