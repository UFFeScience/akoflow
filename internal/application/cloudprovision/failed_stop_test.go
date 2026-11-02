package cloudprovision

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestStopFailedConfigurationPreservesDiskAndFailureEvidence(t *testing.T) {
	instance := domain.CloudProvisionedInstance{ID: "vm", Status: "failed", ProviderID: "123", FailureReason: "host key unavailable", PublicAddress: "203.0.113.10", Disk: map[string]any{"name": "retained-disk"}}
	store := &capacityStoreStub{instances: []domain.CloudProvisionedInstance{instance}}
	terraform := &terraformStub{}
	service := &Provisioner{store: store, terraform: terraform}
	stopped, err := service.Stop(context.Background(), "vm")
	if err != nil || stopped.Status != "stopped" || stopped.Disk["name"] != "retained-disk" || stopped.FailureReason != instance.FailureReason || len(terraform.stopped) != 1 || len(terraform.destroyed) != 0 {
		t.Fatalf("unsafe stop: %+v %v", stopped, err)
	}
}

func TestStopRejectsActiveOrUnmaterializedFailedInstance(t *testing.T) {
	for _, status := range []string{"provisioning", "configuring", "failed"} {
		store := &capacityStoreStub{instances: []domain.CloudProvisionedInstance{{ID: "vm", Status: status}}}
		terraform := &terraformStub{}
		if _, err := (&Provisioner{store: store, terraform: terraform}).Stop(context.Background(), "vm"); err == nil || len(terraform.stopped) != 0 {
			t.Fatalf("accepted %s", status)
		}
	}
}
