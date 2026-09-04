package cloud_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbcloud "github.com/UFFeScience/akoflow/internal/infrastructure/database/cloud"
)

func TestRepositoryStoresConfigurationsAndTargets(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "cloud.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO environments(id,name) VALUES('gcp-env','GCP')`); err != nil {
		t.Fatal(err)
	}
	repository := dbcloud.New(db)
	if err := repository.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	configuration, err := repository.FindMachineConfiguration(ctx, domain.DefaultMachineConfigurationID)
	if err != nil || configuration == nil || len(configuration.Versions) != 1 {
		t.Fatalf("default configuration: %#v, %v", configuration, err)
	}

	if err := repository.CreateCapacityTarget(ctx, domain.CloudCapacityTarget{
		ID: "cpu-8", EnvironmentID: "gcp-env", Name: "CPU 8", Provider: "gcp",
		ProviderMachineType: "c3-standard-8", Region: "us-central1", ImageReference: "ubuntu-2404",
		VCPU: 8, MemoryMiB: 32768, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	targets, err := repository.ListCapacityTargets(ctx, "gcp-env")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || len(targets[0].MachineConfigurations) != 1 || !targets[0].MachineConfigurations[0].Required {
		t.Fatalf("unexpected targets: %#v", targets)
	}
	if err := repository.DeleteCapacityTarget(ctx, "cpu-8"); err != nil {
		t.Fatal(err)
	}
	targets, err = repository.ListCapacityTargets(ctx, "gcp-env")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("removed target remains visible: %#v", targets)
	}
	if err := repository.CreateCapacityTarget(ctx, domain.CloudCapacityTarget{
		ID: "cpu-8-replacement", EnvironmentID: "gcp-env", Name: "CPU 8", Provider: "gcp",
		ProviderMachineType: "c3-standard-8", Region: "us-central1", ImageReference: "ubuntu-2404",
		VCPU: 8, MemoryMiB: 32768, Enabled: true,
	}); err != nil {
		t.Fatalf("reuse removed target name: %v", err)
	}
}

func TestRepositoryRejectsInvalidPlaybook(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "cloud.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := dbcloud.New(db)
	if err := repository.CreateMachineConfiguration(ctx, domain.MachineConfiguration{ID: "custom", Name: "Custom", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	err = repository.CreateMachineConfigurationVersion(ctx, domain.MachineConfigurationVersion{ID: "custom-v1", MachineConfigurationID: "custom", Version: 1, Status: "published", PlaybookYAML: "not: a playbook"})
	if err == nil {
		t.Fatal("expected invalid playbook to be rejected")
	}
}
