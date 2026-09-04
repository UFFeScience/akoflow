package runtime

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type cloudStoreStub struct {
	instances []domain.CloudProvisionedInstance
}

func (*cloudStoreStub) EnsureDefaults(context.Context) error { return nil }
func (*cloudStoreStub) CreateMachineConfiguration(context.Context, domain.MachineConfiguration) error {
	return nil
}
func (*cloudStoreStub) CreateMachineConfigurationVersion(context.Context, domain.MachineConfigurationVersion) error {
	return nil
}
func (*cloudStoreStub) ListMachineConfigurations(context.Context) ([]domain.MachineConfiguration, error) {
	return nil, nil
}
func (*cloudStoreStub) FindMachineConfiguration(context.Context, string) (*domain.MachineConfiguration, error) {
	return nil, nil
}
func (*cloudStoreStub) FindMachineConfigurationVersion(context.Context, string) (*domain.MachineConfigurationVersion, error) {
	return nil, nil
}
func (*cloudStoreStub) CreateCapacityTarget(context.Context, domain.CloudCapacityTarget) error {
	return nil
}
func (*cloudStoreStub) ListCapacityTargets(context.Context, string) ([]domain.CloudCapacityTarget, error) {
	return nil, nil
}
func (*cloudStoreStub) FindCapacityTarget(context.Context, string) (*domain.CloudCapacityTarget, error) {
	return nil, nil
}
func (*cloudStoreStub) DeleteCapacityTarget(context.Context, string) error { return nil }
func (*cloudStoreStub) CreateProvisionedInstance(context.Context, domain.CloudProvisionedInstance) error {
	return nil
}
func (*cloudStoreStub) UpdateProvisionedInstance(context.Context, domain.CloudProvisionedInstance) error {
	return nil
}
func (s *cloudStoreStub) FindProvisionedInstance(_ context.Context, id string) (*domain.CloudProvisionedInstance, error) {
	for index := range s.instances {
		if s.instances[index].ID == id {
			return &s.instances[index], nil
		}
	}
	return nil, nil
}
func (s *cloudStoreStub) ListProvisionedInstances(context.Context, string) ([]domain.CloudProvisionedInstance, error) {
	return s.instances, nil
}

type cloudProvisionerStub struct {
	created domain.CloudProvisionedInstance
	calls   int
}

func (s *cloudProvisionerStub) Provision(context.Context, string, domain.CloudProvisionRequest) (domain.CloudProvisionedInstance, error) {
	s.calls++
	return s.created, nil
}
func (*cloudProvisionerStub) Destroy(context.Context, string) (domain.CloudProvisionedInstance, error) {
	return domain.CloudProvisionedInstance{}, nil
}
func (*cloudProvisionerStub) Release(context.Context, []string) error { return nil }

type executorStub struct {
	responses [][]byte
	calls     int
}

func (s *executorStub) Run(context.Context, string, []string, []byte) ([]byte, error) {
	index := s.calls
	s.calls++
	if index < len(s.responses) {
		return s.responses[index], nil
	}
	return nil, nil
}

func TestCloudRuntimeProvisionsExactTargetAndStartsRemoteDocker(t *testing.T) {
	store := &cloudStoreStub{}
	provisioner := &cloudProvisionerStub{created: domain.CloudProvisionedInstance{
		ID: "instance", CapacityTargetID: "target", EnvironmentID: "environment", Status: "ready",
		PublicAddress: "203.0.113.10", SSHUsername: "akoflow", SSHCredentialRef: "file:/key",
	}}
	executor := &executorStub{responses: [][]byte{nil, []byte("[]"), []byte("container\n")}}
	adapter := &Adapter{environmentID: "environment", store: store, provisioner: provisioner, executor: executor}
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, RuntimeID: "runtime", Resource: domain.Resource{ID: "target"},
		Activity: domain.Activity{ID: "activity", Command: domain.ActivityCommand{
			Image: "ubuntu:latest", Entrypoint: "sh", Arguments: []string{"-c", "echo ok > result.txt"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if provisioner.calls != 1 || handle.Metadata["cloudInstanceId"] != "instance" || handle.ExternalID != "container" {
		t.Fatalf("handle = %#v, provisions = %d", handle, provisioner.calls)
	}
}

func TestCloudRuntimeReusesOnlyReadyInstanceForTarget(t *testing.T) {
	store := &cloudStoreStub{instances: []domain.CloudProvisionedInstance{
		{ID: "wrong", CapacityTargetID: "other", Status: "ready"},
		{ID: "stopped", CapacityTargetID: "target", Status: "stopped"},
		{ID: "ready", CapacityTargetID: "target", Status: "ready"},
	}}
	adapter := &Adapter{environmentID: "environment", store: store, provisioner: &cloudProvisionerStub{}}
	instance, err := adapter.readyInstance(context.Background(), "target")
	if err != nil || instance == nil || instance.ID != "ready" {
		t.Fatalf("instance = %#v, err = %v", instance, err)
	}
}

var _ ports.CloudConfigurationStore = (*cloudStoreStub)(nil)
var _ ports.CloudProvisioner = (*cloudProvisionerStub)(nil)
