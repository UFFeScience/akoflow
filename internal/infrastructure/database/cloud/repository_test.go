package cloud_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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
		ID: "cpu-8", EnvironmentID: "gcp-env", Name: "CPU 8 replacement", Provider: "gcp",
		ProviderMachineType: "c3-standard-8", Region: "us-central1", ImageReference: "ubuntu-2404",
		VCPU: 8, MemoryMiB: 32768, Enabled: true,
		Configuration: map[string]any{"diskType": "hyperdisk-balanced"},
	}); err != nil {
		t.Fatalf("restore removed target id: %v", err)
	}
	targets, err = repository.ListCapacityTargets(ctx, "gcp-env")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].ID != "cpu-8" || targets[0].Name != "CPU 8 replacement" || targets[0].Configuration["diskType"] != "hyperdisk-balanced" {
		t.Fatalf("restored target = %#v", targets)
	}
}

func TestCapacityTargetHonorsPublishedRequiredWorkerVersion(t *testing.T) {
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
	version := domain.MachineConfigurationVersion{ID: "preinstalled-v8", MachineConfigurationID: domain.DefaultMachineConfigurationID,
		Version: 8, Status: "published", PlaybookYAML: "---\n- hosts: all\n  tasks:\n    - name: Verify Docker\n      ansible.builtin.command: docker --version\n      changed_when: false\n"}
	if err := repository.CreateMachineConfigurationVersion(ctx, version); err != nil {
		t.Fatal(err)
	}
	for _, required := range []bool{true, false} {
		target := domain.CloudCapacityTarget{ID: "preinstalled", EnvironmentID: "gcp-env", Name: "Preinstalled worker",
			Provider: "gcp", ProviderMachineType: "e2-micro", ImageReference: "worker-image", VCPU: 1, MemoryMiB: 1024, Enabled: true,
			MachineConfigurations: []domain.CloudTargetConfiguration{{ConfigurationVersionID: version.ID, ExecutionOrder: 0, Required: required, Enabled: true}}}
		if err := repository.CreateCapacityTarget(ctx, target); err != nil {
			t.Fatal(err)
		}
		stored, err := repository.ListCapacityTargets(ctx, "gcp-env")
		if err != nil {
			t.Fatal(err)
		}
		configs := stored[0].MachineConfigurations
		if required {
			if len(configs) != 1 || configs[0].ConfigurationVersionID != version.ID {
				t.Fatalf("explicit baseline not honored: %#v", configs)
			}
		} else {
			if len(configs) != 2 || configs[0].ConfigurationVersionID != domain.DefaultMachineConfigurationVersionID || configs[0].ExecutionOrder >= configs[1].ExecutionOrder {
				t.Fatalf("optional version bypassed baseline or collided: %#v", configs)
			}
		}
	}
}

func TestRepositoryStoresCloudOperationEventsAndExecutionBinding(t *testing.T) {
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
	operation := domain.CloudOperationRun{ID: "operation", Kind: "provision", Status: "running", EnvironmentID: "gcp-env", ExecutionRunID: "run", ActivityID: "activity", Phase: "terraform", CreatedAt: time.Now().UTC()}
	if err := repository.CreateCloudOperation(ctx, operation); err != nil {
		t.Fatal(err)
	}
	if err := repository.AppendCloudOperationEvent(ctx, domain.CloudOperationEvent{OperationID: operation.ID, Sequence: 1, Tool: "terraform", Phase: "apply", Event: "resource_start", Message: "creating VM"}); err != nil {
		t.Fatal(err)
	}
	found, err := repository.FindCloudOperation(ctx, operation.ID)
	if err != nil || found == nil || found.ExecutionRunID != "run" || found.ActivityID != "activity" || found.Phase != "terraform" {
		t.Fatalf("operation = %#v, %v", found, err)
	}
	events, err := repository.ListCloudOperationEvents(ctx, operation.ID)
	if err != nil || len(events) != 1 || events[0].Tool != "terraform" || events[0].Message != "creating VM" {
		t.Fatalf("events = %#v, %v", events, err)
	}
}

func TestRepositoryPreservesInstanceBilling(t *testing.T) {
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
	if err := repository.CreateCapacityTarget(ctx, domain.CloudCapacityTarget{
		ID: "cpu-8", EnvironmentID: "gcp-env", Name: "CPU 8", Provider: "gcp",
		ProviderMachineType: "c3-standard-8", Region: "us-central1", ImageReference: "ubuntu-2404",
		VCPU: 8, MemoryMiB: 32768, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	instance := domain.CloudProvisionedInstance{
		ID: "instance-1", CapacityTargetID: "cpu-8", EnvironmentID: "gcp-env",
		Provider: "gcp", Name: "Machine", Status: "ready", CreatedAt: started,
		TerraformOutput: map[string]any{"zone": "us-central1-a"},
		Billing: &domain.InstanceBilling{StartedAt: started, ComputeStartedAt: &started,
			ComputePricePerSecond: 0.001, DiskPricePerSecond: 0.0001},
	}
	if err := repository.CreateProvisionedInstance(ctx, instance); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.FindProvisionedInstance(ctx, instance.ID)
	if err != nil || loaded == nil || loaded.Billing == nil {
		t.Fatalf("loaded billing = %#v, %v", loaded, err)
	}
	if loaded.Billing.ComputePricePerSecond != 0.001 || loaded.TerraformOutput["_akoflowBilling"] != nil {
		t.Fatalf("invalid billing round trip: %#v", loaded)
	}
	instance.Billing.AccumulatedComputeSeconds = 20
	instance.Billing.ComputeStartedAt = nil
	if err := repository.UpdateProvisionedInstance(ctx, instance); err != nil {
		t.Fatal(err)
	}
	loaded, err = repository.FindProvisionedInstance(ctx, instance.ID)
	if err != nil || loaded.Billing.AccumulatedComputeSeconds != 20 || loaded.Billing.ComputeStartedAt != nil {
		t.Fatalf("updated billing = %#v, %v", loaded, err)
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
