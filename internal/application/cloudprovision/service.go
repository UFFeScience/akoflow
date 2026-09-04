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

type Service struct {
	store        ports.CloudConfigurationStore
	environments ports.EnvironmentCatalog
	credentials  ports.CloudCredentialResolver
	sshKeys      ports.CloudSSHKeyManager
	terraform    ports.TerraformRunner
}

func New(
	store ports.CloudConfigurationStore,
	environments ports.EnvironmentCatalog,
	credentials ports.CloudCredentialResolver,
	sshKeys ports.CloudSSHKeyManager,
	terraform ports.TerraformRunner,
) *Service {
	return &Service{
		store: store, environments: environments, credentials: credentials,
		sshKeys: sshKeys, terraform: terraform,
	}
}

func (s *Service) Provision(
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
	now := time.Now().UTC()
	instance.Status = "ready"
	instance.ProviderID = result.ProviderID
	instance.PublicAddress = result.PublicAddress
	instance.PrivateAddress = result.PrivateAddress
	instance.Disk = result.Disk
	instance.TerraformOutput = result.Output
	instance.ReadyAt = &now
	if err := s.store.UpdateProvisionedInstance(ctx, instance); err != nil {
		return instance, err
	}
	return instance, nil
}

func (s *Service) Destroy(ctx context.Context, instanceID string) (domain.CloudProvisionedInstance, error) {
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
