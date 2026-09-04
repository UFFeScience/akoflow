package cloudprovision

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/google/uuid"
)

type Provisioner struct {
	store        ports.CloudConfigurationStore
	environments ports.EnvironmentCatalog
	credentials  ports.CloudCredentialResolver
	sshKeys      ports.CloudSSHKeyManager
	terraform    ports.TerraformRunner
	configurator ports.MachineConfigurator
}

func New(
	store ports.CloudConfigurationStore,
	environments ports.EnvironmentCatalog,
	credentials ports.CloudCredentialResolver,
	sshKeys ports.CloudSSHKeyManager,
	terraform ports.TerraformRunner,
	configurator ports.MachineConfigurator,
) *Provisioner {
	return &Provisioner{
		store: store, environments: environments, credentials: credentials,
		sshKeys: sshKeys, terraform: terraform, configurator: configurator,
	}
}

func (s *Provisioner) Provision(
	ctx context.Context,
	environmentID string,
	request domain.CloudProvisionRequest,
) (domain.CloudProvisionedInstance, error) {
	target, err := s.store.FindCapacityTarget(ctx, request.CapacityTargetID)
	if err != nil || target == nil {
		return domain.CloudProvisionedInstance{}, targetError(request.CapacityTargetID, err)
	}
	if target.EnvironmentID != environmentID {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("capacity target does not belong to environment")
	}
	definition, err := s.environments.Find(ctx, environmentID)
	if err != nil || definition == nil {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("load cloud environment: %w", err)
	}
	connection := cloudConnection(definition.Connections)
	if connection == nil {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("environment has no cloud connection")
	}
	if instance, handled, reuseErr := s.reuseCapacity(ctx, *target); handled {
		return instance, reuseErr
	}
	credential, err := s.credentials.Resolve(connection.CredentialRef)
	if err != nil {
		return domain.CloudProvisionedInstance{}, err
	}
	instanceID := "cloud-instance-" + uuid.NewString()
	sshUser := strings.TrimSpace(request.SSHUsername)
	if sshUser == "" {
		sshUser = "akoflow"
	}
	key, err := s.sshKeys.Ensure(instanceID, "akoflow "+instanceID)
	if err != nil {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("prepare SSH identity: %w", err)
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = instanceID
	}
	instance := domain.CloudProvisionedInstance{
		ID: instanceID, CapacityTargetID: target.ID, EnvironmentID: environmentID,
		Provider: target.Provider, Name: name, Status: "provisioning", SSHUsername: sshUser,
		SSHCredentialRef: key.CredentialRef, CreatedAt: time.Now().UTC(),
	}
	if err := s.store.CreateProvisionedInstance(ctx, instance); err != nil {
		return domain.CloudProvisionedInstance{}, err
	}
	result, err := s.terraform.Apply(ctx, ports.TerraformProvisionSpec{
		InstanceID: instanceID, Target: *target, Credential: credential,
		PublicKey: key.PublicKey, SSHUser: sshUser,
	})
	if err != nil {
		instance.Status = "failed"
		instance.FailureReason = err.Error()
		_ = s.store.UpdateProvisionedInstance(ctx, instance)
		return instance, err
	}
	return s.finishProvision(ctx, instance, *target, result)
}

func (s *Provisioner) finishProvision(
	ctx context.Context,
	instance domain.CloudProvisionedInstance,
	target domain.CloudCapacityTarget,
	result ports.TerraformResult,
) (domain.CloudProvisionedInstance, error) {
	instance.ProviderID = result.ProviderID
	instance.PublicAddress = result.PublicAddress
	instance.PrivateAddress = result.PrivateAddress
	instance.Disk = result.Disk
	instance.TerraformOutput = result.Output
	instance.Status = "configuring"
	if err := s.store.UpdateProvisionedInstance(ctx, instance); err != nil {
		return instance, err
	}
	if err := s.configure(ctx, &instance, target, target.MachineConfigurations); err != nil {
		instance.Status = "failed"
		instance.FailureReason = err.Error()
		_ = s.store.UpdateProvisionedInstance(ctx, instance)
		return instance, err
	}
	now := time.Now().UTC()
	instance.Status = "ready"
	instance.ReadyAt = &now
	if err := s.store.UpdateProvisionedInstance(ctx, instance); err != nil {
		return instance, err
	}
	return instance, nil
}

func (s *Provisioner) reuseCapacity(
	ctx context.Context,
	target domain.CloudCapacityTarget,
) (domain.CloudProvisionedInstance, bool, error) {
	instances, err := s.store.ListProvisionedInstances(ctx, target.EnvironmentID)
	if err != nil {
		return domain.CloudProvisionedInstance{}, true, err
	}
	active := 0
	for index := range instances {
		instance := &instances[index]
		if instance.CapacityTargetID != target.ID || instance.Status == "destroyed" || instance.Status == "failed" {
			continue
		}
		active++
		if instance.Status != "stopped" {
			continue
		}
		result, startErr := s.terraform.Start(ctx, instance.ID)
		if startErr != nil {
			instance.Status, instance.FailureReason = "failed", startErr.Error()
			_ = s.store.UpdateProvisionedInstance(ctx, *instance)
			return *instance, true, startErr
		}
		instance.Status, instance.FailureReason = "configuring", ""
		instance.PublicAddress, instance.PrivateAddress = result.PublicAddress, result.PrivateAddress
		instance.TerraformOutput, instance.Disk = result.Output, result.Disk
		if configureErr := s.configure(ctx, instance, target, target.MachineConfigurations); configureErr != nil {
			instance.Status, instance.FailureReason = "failed", configureErr.Error()
			_ = s.store.UpdateProvisionedInstance(ctx, *instance)
			return *instance, true, configureErr
		}
		now := time.Now().UTC()
		instance.Status, instance.ReadyAt = "ready", &now
		if updateErr := s.store.UpdateProvisionedInstance(ctx, *instance); updateErr != nil {
			return *instance, true, updateErr
		}
		return *instance, true, nil
	}
	limit := target.MaximumInstances
	if limit <= 0 {
		limit = 1
	}
	if active >= limit {
		return domain.CloudProvisionedInstance{}, true, fmt.Errorf(
			"capacity target %q already has %d of %d instances allocated", target.ID, active, limit,
		)
	}
	return domain.CloudProvisionedInstance{}, false, nil
}

func (s *Provisioner) configure(
	ctx context.Context,
	instance *domain.CloudProvisionedInstance,
	target domain.CloudCapacityTarget,
	configurations []domain.CloudTargetConfiguration,
) error {
	for _, assignment := range configurations {
		if !assignment.Enabled {
			continue
		}
		version, err := s.store.FindMachineConfigurationVersion(ctx, assignment.ConfigurationVersionID)
		if err != nil || version == nil {
			if assignment.Required {
				return fmt.Errorf("load required machine configuration %q: %w", assignment.ConfigurationVersionID, err)
			}
			continue
		}
		if compatibilityErr := validateCompatibility(*version, target, assignment.Required); compatibilityErr != nil {
			return compatibilityErr
		}
		variables := map[string]any{"akoflow_workspace_path": "/akoflow/workspace"}
		for key, value := range assignment.Variables {
			variables[key] = value
		}
		err = s.configurator.Configure(ctx, ports.MachineConfigurationSpec{
			InstanceID: instance.ID, Address: instance.PublicAddress,
			SSHUser: instance.SSHUsername, CredentialRef: instance.SSHCredentialRef,
			PlaybookYAML: version.PlaybookYAML, Variables: variables,
			Checks: version.ValidationChecks,
		})
		if err != nil && assignment.Required {
			return fmt.Errorf("apply machine configuration %q: %w", assignment.ConfigurationVersionID, err)
		}
	}
	return nil
}

func (s *Provisioner) Configure(ctx context.Context, instanceID string) (domain.CloudProvisionedInstance, error) {
	instance, err := s.store.FindProvisionedInstance(ctx, instanceID)
	if err != nil || instance == nil {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("cloud instance %q was not found", instanceID)
	}
	if strings.TrimSpace(instance.PublicAddress) == "" {
		return *instance, fmt.Errorf("cloud instance %q has no public address", instanceID)
	}
	target, err := s.store.FindCapacityTarget(ctx, instance.CapacityTargetID)
	if err != nil || target == nil {
		return *instance, targetError(instance.CapacityTargetID, err)
	}
	instance.Status, instance.FailureReason = "configuring", ""
	if err := s.store.UpdateProvisionedInstance(ctx, *instance); err != nil {
		return *instance, err
	}
	if err := s.configure(ctx, instance, *target, target.MachineConfigurations); err != nil {
		instance.Status, instance.FailureReason = "failed", err.Error()
		_ = s.store.UpdateProvisionedInstance(ctx, *instance)
		return *instance, err
	}
	now := time.Now().UTC()
	instance.Status, instance.ReadyAt = "ready", &now
	if err := s.store.UpdateProvisionedInstance(ctx, *instance); err != nil {
		return *instance, err
	}
	return *instance, nil
}

func validateCompatibility(
	version domain.MachineConfigurationVersion,
	target domain.CloudCapacityTarget,
	required bool,
) error {
	compatible := func(accepted []string, actual string) bool {
		if len(accepted) == 0 {
			return true
		}
		for _, value := range accepted {
			if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(actual)) {
				return true
			}
		}
		return false
	}
	if compatible(version.Compatibility.Providers, target.Provider) &&
		compatible(version.Compatibility.Architectures, target.Architecture) {
		return nil
	}
	if !required {
		return nil
	}
	return fmt.Errorf(
		"required machine configuration %q is not compatible with provider %q and architecture %q",
		version.ID, target.Provider, target.Architecture,
	)
}

func (s *Provisioner) Destroy(ctx context.Context, instanceID string) (domain.CloudProvisionedInstance, error) {
	instance, err := s.store.FindProvisionedInstance(ctx, instanceID)
	if err != nil || instance == nil {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("cloud instance %q was not found", instanceID)
	}
	instance.Status = "destroying"
	if err := s.store.UpdateProvisionedInstance(ctx, *instance); err != nil {
		return *instance, err
	}
	if err := s.terraform.Destroy(ctx, instanceID); err != nil {
		instance.Status = "failed"
		instance.FailureReason = err.Error()
		_ = s.store.UpdateProvisionedInstance(ctx, *instance)
		return *instance, err
	}
	now := time.Now().UTC()
	instance.Status = "destroyed"
	instance.DestroyedAt = &now
	instance.PublicAddress = ""
	instance.PrivateAddress = ""
	if err := s.store.UpdateProvisionedInstance(ctx, *instance); err != nil {
		return *instance, err
	}
	return *instance, nil
}

func (s *Provisioner) Release(ctx context.Context, capacityTargetIDs []string) error {
	selected := make(map[string]bool, len(capacityTargetIDs))
	for _, id := range capacityTargetIDs {
		selected[id] = true
	}
	for targetID := range selected {
		target, err := s.store.FindCapacityTarget(ctx, targetID)
		if err != nil || target == nil {
			return targetError(targetID, err)
		}
		instances, err := s.store.ListProvisionedInstances(ctx, target.EnvironmentID)
		if err != nil {
			return err
		}
		for index := range instances {
			instance := &instances[index]
			if instance.CapacityTargetID != targetID || instance.Status != "ready" {
				continue
			}
			switch target.LifecyclePolicy {
			case "destroy-after-run", "destroy-after-execution":
				if _, err := s.Destroy(ctx, instance.ID); err != nil {
					return err
				}
			case "stop-when-idle":
				if err := s.terraform.Stop(ctx, instance.ID); err != nil {
					return err
				}
				instance.Status = "stopped"
				instance.PublicAddress = ""
				if err := s.store.UpdateProvisionedInstance(ctx, *instance); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Provisioner) Log(ctx context.Context, instanceID string) ([]byte, error) {
	instance, err := s.store.FindProvisionedInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance == nil {
		return nil, fmt.Errorf("cloud instance %q was not found", instanceID)
	}
	return s.terraform.Log(ctx, instanceID)
}

func cloudConnection(connections []domain.EnvironmentConnection) *domain.EnvironmentConnection {
	for index := range connections {
		if connections[index].Type == domain.ConnectionCloud {
			return &connections[index]
		}
	}
	return nil
}

func targetError(id string, err error) error {
	if err != nil {
		return fmt.Errorf("load capacity target: %w", err)
	}
	return fmt.Errorf("capacity target %q was not found", id)
}
