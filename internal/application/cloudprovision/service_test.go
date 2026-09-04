package cloudprovision

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type capacityStoreStub struct {
	instances []domain.CloudProvisionedInstance
	updated   []domain.CloudProvisionedInstance
}

func (*capacityStoreStub) EnsureDefaults(context.Context) error { return nil }
func (*capacityStoreStub) CreateMachineConfiguration(context.Context, domain.MachineConfiguration) error {
	return nil
}
func (*capacityStoreStub) CreateMachineConfigurationVersion(context.Context, domain.MachineConfigurationVersion) error {
	return nil
}
func (*capacityStoreStub) ListMachineConfigurations(context.Context) ([]domain.MachineConfiguration, error) {
	return nil, nil
}
func (*capacityStoreStub) FindMachineConfiguration(context.Context, string) (*domain.MachineConfiguration, error) {
	return nil, nil
}
func (*capacityStoreStub) FindMachineConfigurationVersion(context.Context, string) (*domain.MachineConfigurationVersion, error) {
	return nil, nil
}
func (*capacityStoreStub) CreateCapacityTarget(context.Context, domain.CloudCapacityTarget) error {
	return nil
}
func (*capacityStoreStub) ListCapacityTargets(context.Context, string) ([]domain.CloudCapacityTarget, error) {
	return nil, nil
}
func (*capacityStoreStub) FindCapacityTarget(context.Context, string) (*domain.CloudCapacityTarget, error) {
	return nil, nil
}
func (*capacityStoreStub) DeleteCapacityTarget(context.Context, string) error { return nil }
func (*capacityStoreStub) CreateProvisionedInstance(context.Context, domain.CloudProvisionedInstance) error {
	return nil
}
func (s *capacityStoreStub) UpdateProvisionedInstance(_ context.Context, instance domain.CloudProvisionedInstance) error {
	s.updated = append(s.updated, instance)
	return nil
}
func (*capacityStoreStub) FindProvisionedInstance(context.Context, string) (*domain.CloudProvisionedInstance, error) {
	return nil, nil
}
func (s *capacityStoreStub) ListProvisionedInstances(context.Context, string) ([]domain.CloudProvisionedInstance, error) {
	return s.instances, nil
}

type terraformStub struct {
	started []string
	result  ports.TerraformResult
}

func (*terraformStub) Apply(context.Context, ports.TerraformProvisionSpec) (ports.TerraformResult, error) {
	return ports.TerraformResult{}, nil
}
func (s *terraformStub) Start(_ context.Context, id string) (ports.TerraformResult, error) {
	s.started = append(s.started, id)
	return s.result, nil
}
func (*terraformStub) Destroy(context.Context, string) error       { return nil }
func (*terraformStub) Stop(context.Context, string) error          { return nil }
func (*terraformStub) Log(context.Context, string) ([]byte, error) { return nil, nil }

func TestReuseCapacityResumesStoppedTerraformInstance(t *testing.T) {
	store := &capacityStoreStub{instances: []domain.CloudProvisionedInstance{{
		ID: "instance", EnvironmentID: "environment", CapacityTargetID: "target", Status: "stopped",
	}}}
	terra := &terraformStub{result: ports.TerraformResult{
		PublicAddress: "203.0.113.20", PrivateAddress: "10.0.0.20",
	}}
	service := &Provisioner{store: store, terraform: terra}
	instance, handled, err := service.reuseCapacity(context.Background(), domain.CloudCapacityTarget{
		ID: "target", EnvironmentID: "environment", MaximumInstances: 1,
	})
	if err != nil || !handled {
		t.Fatalf("handled = %v, err = %v", handled, err)
	}
	if instance.Status != "ready" || instance.PublicAddress != "203.0.113.20" || len(terra.started) != 1 {
		t.Fatalf("instance = %#v, started = %#v", instance, terra.started)
	}
}

func TestReuseCapacityEnforcesMaximumAllocatedInstances(t *testing.T) {
	store := &capacityStoreStub{instances: []domain.CloudProvisionedInstance{{
		ID: "instance", EnvironmentID: "environment", CapacityTargetID: "target", Status: "ready",
	}}}
	service := &Provisioner{store: store, terraform: &terraformStub{}}
	_, handled, err := service.reuseCapacity(context.Background(), domain.CloudCapacityTarget{
		ID: "target", EnvironmentID: "environment", MaximumInstances: 1,
	})
	if !handled || err == nil {
		t.Fatalf("handled = %v, err = %v", handled, err)
	}
}

func TestRequiredConfigurationMustMatchProviderAndArchitecture(t *testing.T) {
	version := domain.MachineConfigurationVersion{
		ID: "config", Compatibility: domain.MachineConfigurationCompatibility{
			Providers: []string{"gcp"}, Architectures: []string{"amd64"},
		},
	}
	if err := validateCompatibility(version, domain.CloudCapacityTarget{
		Provider: "gcp", Architecture: "amd64",
	}, true); err != nil {
		t.Fatal(err)
	}
	if err := validateCompatibility(version, domain.CloudCapacityTarget{
		Provider: "aws", Architecture: "amd64",
	}, true); err == nil {
		t.Fatal("expected incompatible provider to be rejected")
	}
	if err := validateCompatibility(version, domain.CloudCapacityTarget{
		Provider: "gcp", Architecture: "arm64",
	}, true); err == nil {
		t.Fatal("expected incompatible architecture to be rejected")
	}
}

var _ ports.CloudConfigurationStore = (*capacityStoreStub)(nil)
var _ ports.TerraformRunner = (*terraformStub)(nil)
