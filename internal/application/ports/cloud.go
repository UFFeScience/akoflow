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
	CreateCapacityTarget(context.Context, domain.CloudCapacityTarget) error
	ListCapacityTargets(context.Context, string) ([]domain.CloudCapacityTarget, error)
}

type CloudCredentialResolver interface {
	Resolve(string) ([]byte, error)
}

type CloudCatalogProvider interface {
	Provider() string
	Discover(context.Context, domain.EnvironmentConnection, []byte) (domain.CloudCatalog, error)
}

type CloudCatalog interface {
	Discover(context.Context, string) (domain.CloudCatalog, error)
}
