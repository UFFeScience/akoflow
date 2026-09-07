package algorithms

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestAddCloudLifecycleAccountsForPreparationIdleAndDisk(t *testing.T) {
	resource := planningResource("cloud", 2, 1, 0.001)
	resource.Type = domain.ResourceCloudVM
	resource.StorageBytes = 100 << 30
	resource.Metadata = map[string]any{"terraformSeconds": 10.0, "sshWaitSeconds": 5.0, "configurationSeconds": 10.0, "validationSeconds": 5.0, "destroySeconds": 5.0, "diskPricePerGiBMonth": 0.1}
	request := domain.PlanningRequest{Resources: []domain.Resource{resource}}
	plan := domain.SchedulePlan{ID: "plan", Predicted: domain.PredictedMetrics{Cost: 0.01, Feasible: true}, Assignments: []domain.PlanAssignment{{ID: "activity", ResourceID: resource.ID, PredictedStartAt: 30, PredictedFinishAt: 40, PredictedRuntimeSeconds: 10}}}
	addCloudLifecycle(&plan, request)
	if len(plan.LifecycleActions) != 4 {
		t.Fatalf("actions = %#v", plan.LifecycleActions)
	}
	if plan.LifecycleActions[0].Action != "provision" || plan.LifecycleActions[0].EarliestStart != 0 || plan.LifecycleActions[3].Action != "destroy" || plan.LifecycleActions[3].EarliestStart != 40 {
		t.Fatalf("actions = %#v", plan.LifecycleActions)
	}
	if plan.Predicted.Cost <= 0.01 || plan.Metadata["cloudCost.cloud"] == nil {
		t.Fatalf("prediction = %#v metadata=%#v", plan.Predicted, plan.Metadata)
	}
}
