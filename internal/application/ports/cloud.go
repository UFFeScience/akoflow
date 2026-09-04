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
