package execution

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestRecoveredAllocationPreservesExactProducerInstance(t *testing.T) {
	task := domain.TaskExecution{AllocatedResourceID: "target", PlannedResourceID: "other", RuntimeID: "cloud", ConnectionID: "connection", CloudInstanceID: "producer-vm"}
	allocation := recoveredAllocation(task)
	if allocation.ResourceID != "target" || allocation.CloudInstanceID != "producer-vm" || allocation.RuntimeID != "cloud" || allocation.ConnectionID != "connection" {
		t.Fatalf("allocation lost exact producer identity: %+v", allocation)
	}
	task.AllocatedResourceID = ""
	if recoveredAllocation(task).ResourceID != "other" {
		t.Fatal("planned resource fallback was lost")
	}
}
