package simgrid

import (
	"math"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestCloudLifecycleCostsUsesSimulatedFinishAndBillsIdleOnce(t *testing.T) {
	resource := domain.Resource{ID: "cloud", Type: domain.ResourceCloudVM, PricePerSecond: 0.01, StorageBytes: 30 << 30, Metadata: map[string]any{"diskPricePerGiBMonth": 0.1}}
	tasks := []domain.TaskExecution{{AllocatedResourceID: "cloud", StartedAt: 20, FinishedAt: 30, RuntimeSeconds: 10}, {AllocatedResourceID: "cloud", StartedAt: 80, FinishedAt: 100, RuntimeSeconds: 20}}
	actions := []domain.PlannedLifecycleAction{{CapacityTargetID: "cloud", Action: "provision", EarliestStart: 5}, {CapacityTargetID: "cloud", Action: "stop", EarliestStart: 9999, ExpectedDuration: 5}}
	applyCloudLifecycleCosts(tasks, []domain.Resource{resource}, actions)
	expected := 1.0 + 30*0.1*100/(730*3600)
	if math.Abs(tasks[0].Cost+tasks[1].Cost-expected) > 1e-12 {
		t.Fatalf("cost=%g want=%g", tasks[0].Cost+tasks[1].Cost, expected)
	}
	applyCloudLifecycleCosts(tasks, []domain.Resource{resource}, actions)
	if math.Abs(tasks[0].Cost+tasks[1].Cost-expected) > 1e-12 {
		t.Fatal("billing must be idempotent")
	}
}
