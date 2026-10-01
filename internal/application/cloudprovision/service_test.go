package cloudprovision

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type capacityStoreStub struct {
	instances []domain.CloudProvisionedInstance
	updated   []domain.CloudProvisionedInstance
	target    *domain.CloudCapacityTarget
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
func (s *capacityStoreStub) FindCapacityTarget(_ context.Context, id string) (*domain.CloudCapacityTarget, error) {
	if s.target != nil && s.target.ID == id {
		return s.target, nil
	}
	return nil, nil
}
func (*capacityStoreStub) DeleteCapacityTarget(context.Context, string) error { return nil }
func (*capacityStoreStub) CreateProvisionedInstance(context.Context, domain.CloudProvisionedInstance) error {
	return nil
}
func (s *capacityStoreStub) UpdateProvisionedInstance(_ context.Context, instance domain.CloudProvisionedInstance) error {
	s.updated = append(s.updated, instance)
	for index := range s.instances {
		if s.instances[index].ID == instance.ID {
			s.instances[index] = instance
			return nil
		}
	}
	s.instances = append(s.instances, instance)
	return nil
}
func (s *capacityStoreStub) FindProvisionedInstance(_ context.Context, id string) (*domain.CloudProvisionedInstance, error) {
	for index := range s.instances {
		if s.instances[index].ID == id {
			instance := s.instances[index]
			return &instance, nil
		}
	}
	return nil, nil
}
func (s *capacityStoreStub) ListProvisionedInstances(context.Context, string) ([]domain.CloudProvisionedInstance, error) {
	return s.instances, nil
}

type terraformStub struct {
	started   []string
	stopped   []string
	destroyed []string
	result    ports.TerraformResult
	err       error
}

type environmentCatalogStub struct {
	definition domain.EnvironmentDefinition
}

func (s *environmentCatalogStub) Find(context.Context, string) (*domain.EnvironmentDefinition, error) {
	return &s.definition, nil
}
func (*environmentCatalogStub) Create(context.Context, domain.EnvironmentDefinition) error {
	return nil
}
func (*environmentCatalogStub) Replace(context.Context, domain.EnvironmentDefinition) error {
	return nil
}
func (*environmentCatalogStub) Delete(context.Context, string) error { return nil }
func (*environmentCatalogStub) List(context.Context) ([]domain.EnvironmentDefinition, error) {
	return nil, nil
}
func (*environmentCatalogStub) ListConnections(context.Context, string) ([]domain.EnvironmentConnection, error) {
	return nil, nil
}
func (*environmentCatalogStub) UpdateStatus(context.Context, string, domain.EnvironmentStatus) error {
	return nil
}
func (*environmentCatalogStub) UpsertConnection(context.Context, domain.EnvironmentConnection) error {
	return nil
}

type credentialStub struct{}

func (credentialStub) Resolve(string) ([]byte, error) { return []byte("credential"), nil }

type sshKeyStub struct{}

func (sshKeyStub) Ensure(instanceID, _ string) (ports.CloudSSHKey, error) {
	return ports.CloudSSHKey{CredentialRef: "file:/tmp/key", PublicKey: "ssh-ed25519 test"}, nil
}

type configuratorStub struct{}

func (configuratorStub) Configure(context.Context, ports.MachineConfigurationSpec) error { return nil }

func (s *terraformStub) Apply(context.Context, ports.TerraformProvisionSpec) (ports.TerraformResult, error) {
	return s.result, s.err
}
func (s *terraformStub) Start(_ context.Context, id string) (ports.TerraformResult, error) {
	s.started = append(s.started, id)
	return s.result, s.err
}
func (s *terraformStub) Destroy(_ context.Context, id string) error {
	s.destroyed = append(s.destroyed, id)
	return s.err
}
func (s *terraformStub) Stop(_ context.Context, id string) error {
	s.stopped = append(s.stopped, id)
	return s.err
}
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

func TestLifecycleTransitionsPersistTerraformState(t *testing.T) {
	instance := domain.CloudProvisionedInstance{
		ID: "instance", CapacityTargetID: "target", Status: "ready",
		PublicAddress: "203.0.113.20", PrivateAddress: "10.0.0.20",
		TerraformOutput: map[string]any{"zone": "us-central1-a"},
	}
	store := &capacityStoreStub{instances: []domain.CloudProvisionedInstance{instance}, target: &domain.CloudCapacityTarget{ID: "target"}}
	terra := &terraformStub{result: ports.TerraformResult{
		PublicAddress: "203.0.113.21", PrivateAddress: "10.0.0.21",
		Output: map[string]any{"instance_id": "provider-1"},
	}}
	service := &Provisioner{store: store, terraform: terra}

	stopped, err := service.Stop(context.Background(), "instance")
	if err != nil || stopped.Status != "stopped" || len(terra.stopped) != 1 {
		t.Fatalf("stop = %#v, err = %v, calls = %#v", stopped, err, terra.stopped)
	}
	started, err := service.Start(context.Background(), "instance")
	if err != nil || started.Status != "ready" || started.PublicAddress != "203.0.113.21" || started.TerraformOutput["zone"] != "us-central1-a" {
		t.Fatalf("start = %#v, err = %v", started, err)
	}
	destroyed, err := service.Destroy(context.Background(), "instance")
	if err != nil || destroyed.Status != "destroyed" || destroyed.PublicAddress != "" || len(terra.destroyed) != 1 {
		t.Fatalf("destroy = %#v, err = %v, calls = %#v", destroyed, err, terra.destroyed)
	}
}

func TestLifecycleGuardsRejectInvalidStates(t *testing.T) {
	store := &capacityStoreStub{instances: []domain.CloudProvisionedInstance{{ID: "ready", Status: "ready"}, {ID: "busy", Status: "configuring"}, {ID: "stopped", Status: "stopped"}}}
	service := &Provisioner{store: store, terraform: &terraformStub{}}
	if instance, err := service.Start(context.Background(), "ready"); err != nil || instance.Status != "ready" {
		t.Fatalf("expected ready start to be idempotent, got %#v, %v", instance, err)
	}
	if _, err := service.Stop(context.Background(), "busy"); err == nil {
		t.Fatal("expected non-ready stop to be rejected")
	}
	if _, err := service.Start(context.Background(), "busy"); err == nil {
		t.Fatal("expected non-stopped start to be rejected")
	}
}

func TestProvisionRunsTerraformAndPersistsReadyInstance(t *testing.T) {
	store := &capacityStoreStub{target: &domain.CloudCapacityTarget{ID: "target", EnvironmentID: "environment", Provider: "gcp"}}
	terraform := &terraformStub{result: ports.TerraformResult{ProviderID: "provider-1", PublicAddress: "203.0.113.30", PrivateAddress: "10.0.0.30"}}
	environment := &environmentCatalogStub{definition: domain.EnvironmentDefinition{Connections: []domain.EnvironmentConnection{{Type: domain.ConnectionCloud, CredentialRef: "credential-ref"}}}}
	service := New(store, environment, credentialStub{}, sshKeyStub{}, terraform, configuratorStub{})

	instance, err := service.Provision(context.Background(), "environment", domain.CloudProvisionRequest{CapacityTargetID: "target"})
	if err != nil {
		t.Fatalf("provision failed: %v", err)
	}
	if instance.Status != "ready" || instance.ProviderID != "provider-1" || instance.PublicAddress != "203.0.113.30" {
		t.Fatalf("unexpected provisioned instance: %#v", instance)
	}
	if len(store.updated) < 2 {
		t.Fatalf("expected provisioning and ready persistence updates, got %d", len(store.updated))
	}
}

func TestProvisionMarksInstanceFailedWhenTerraformFails(t *testing.T) {
	store := &capacityStoreStub{target: &domain.CloudCapacityTarget{ID: "target", EnvironmentID: "environment", Provider: "gcp"}}
	terraform := &terraformStub{err: errors.New("terraform unavailable")}
	environment := &environmentCatalogStub{definition: domain.EnvironmentDefinition{Connections: []domain.EnvironmentConnection{{Type: domain.ConnectionCloud, CredentialRef: "credential-ref"}}}}
	service := New(store, environment, credentialStub{}, sshKeyStub{}, terraform, configuratorStub{})

	instance, err := service.Provision(context.Background(), "environment", domain.CloudProvisionRequest{CapacityTargetID: "target", InstanceID: "cloud-instance-fixed"})
	if err == nil || instance.Status != "failed" || instance.FailureReason != "terraform unavailable" {
		t.Fatalf("expected failed provision, got %#v, %v", instance, err)
	}
}

var _ ports.CloudConfigurationStore = (*capacityStoreStub)(nil)
var _ ports.TerraformRunner = (*terraformStub)(nil)
