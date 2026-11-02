package storage

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type concreteCloudStore struct {
	ports.CloudConfigurationStore
	instance  *domain.CloudProvisionedInstance
	requested string
}

func (s *concreteCloudStore) FindProvisionedInstance(_ context.Context, id string) (*domain.CloudProvisionedInstance, error) {
	s.requested = id
	return s.instance, nil
}

func TestCloudWorkspaceCatalogRequiresExactReadyTrustedGeneration(t *testing.T) {
	instance := &domain.CloudProvisionedInstance{ID: "vm", ProviderID: "generation", Status: "ready",
		TerraformOutput: map[string]any{"verifiedHostKeyAlias": "alias", "verifiedKnownHostsFile": "known", "verifiedHostGeneration": "generation"}}
	store := &concreteCloudStore{instance: instance}
	catalog := CloudWorkspaceCatalog{Cloud: store}
	storage, err := catalog.FindStorage(context.Background(), "cloud-workspace:vm")
	if err != nil || store.requested != "vm" || storage.Endpoint != "/akoflow/workspace" || len(storage.BrowseRoots) != 1 {
		t.Fatalf("storage=%v err=%v", storage, err)
	}
	for _, state := range []string{"destroyed", "stopped", "configuring"} {
		instance.Status = state
		if _, err := catalog.FindStorage(context.Background(), "cloud-workspace:vm"); err == nil {
			t.Fatalf("accepted %s instance", state)
		}
	}
	instance.Status, instance.ProviderID = "ready", "replacement"
	if _, err := catalog.FindStorage(context.Background(), "cloud-workspace:vm"); err == nil {
		t.Fatal("accepted trust from a different provider generation")
	}
}
