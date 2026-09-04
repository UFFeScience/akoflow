package runtime

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/remote"
)

type Factory struct {
	Store       ports.CloudConfigurationStore
	Provisioner ports.CloudProvisioner
	Executor    provider.CommandExecutor
}

func (Factory) Driver() domain.RuntimeDriver { return domain.RuntimeDriverCloud }

func (f Factory) Build(
	_ domain.EnvironmentRuntime,
	connection domain.EnvironmentConnection,
) (ports.RuntimeAdapter, error) {
	if connection.Type != domain.ConnectionCloud {
		return nil, fmt.Errorf("cloud runtime requires a cloud connection")
	}
	return &Adapter{
		environmentID: connection.EnvironmentID, store: f.Store,
		provisioner: f.Provisioner, executor: f.Executor,
	}, nil
}

type Adapter struct {
	environmentID string
	store         ports.CloudConfigurationStore
	provisioner   ports.CloudProvisioner
	executor      provider.CommandExecutor
}

func (*Adapter) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal}
}

func (a *Adapter) Start(
	ctx context.Context,
	execution domain.ActivityExecutionContext,
) (domain.ActivityHandle, error) {
	instance, err := a.readyInstance(ctx, execution.Resource.ID)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	delegate, err := a.remoteAdapter(*instance)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	handle, err := delegate.Start(ctx, execution)
	if err != nil {
		return handle, err
	}
	if handle.Metadata == nil {
		handle.Metadata = map[string]any{}
	}
	handle.Metadata["cloudInstanceId"] = instance.ID
	return handle, nil
}

func (a *Adapter) Inspect(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	delegate, err := a.delegateForHandle(ctx, handle)
	if err != nil {
		return handle, err
	}
	return delegate.Inspect(ctx, handle)
}

func (a *Adapter) Stop(ctx context.Context, handle domain.ActivityHandle) error {
	delegate, err := a.delegateForHandle(ctx, handle)
	if err != nil {
		return err
	}
	return delegate.Stop(ctx, handle)
}

func (a *Adapter) readyInstance(
	ctx context.Context,
	capacityTargetID string,
) (*domain.CloudProvisionedInstance, error) {
	instances, err := a.store.ListProvisionedInstances(ctx, a.environmentID)
	if err != nil {
		return nil, err
	}
	for index := range instances {
		if instances[index].Status == "ready" && instances[index].CapacityTargetID == capacityTargetID {
			return &instances[index], nil
		}
	}
	created, err := a.provisioner.Provision(ctx, a.environmentID, domain.CloudProvisionRequest{
		CapacityTargetID: capacityTargetID,
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (a *Adapter) delegateForHandle(
	ctx context.Context,
	handle domain.ActivityHandle,
) (ports.RuntimeAdapter, error) {
	instanceID, _ := handle.Metadata["cloudInstanceId"].(string)
	if instanceID == "" {
		return nil, fmt.Errorf("activity handle has no cloud instance")
	}
	instance, err := a.store.FindProvisionedInstance(ctx, instanceID)
	if err != nil || instance == nil {
		return nil, fmt.Errorf("load cloud instance %q: %w", instanceID, err)
	}
	return a.remoteAdapter(*instance)
}

func (a *Adapter) remoteAdapter(instance domain.CloudProvisionedInstance) (ports.RuntimeAdapter, error) {
	connection := domain.EnvironmentConnection{
		ID: a.environmentID + "-" + instance.ID, EnvironmentID: a.environmentID,
		Name: instance.Name, Type: domain.ConnectionSSH, Endpoint: instance.PublicAddress,
		Username: instance.SSHUsername, CredentialRef: instance.SSHCredentialRef,
		Configuration: map[string]any{"adapter": "ssh-docker", "port": 22},
	}
	return (remote.Factory{Executor: a.executor}).Build(domain.EnvironmentRuntime{}, connection)
}
