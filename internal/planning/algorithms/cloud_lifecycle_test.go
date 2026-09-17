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
	provision := plan.LifecycleActions[0]
	stop := plan.LifecycleActions[3]
	if provision.Action != "provision" || provision.EarliestStart != 0 {
		t.Fatalf("provision action = %#v", provision)
	}
	if stop.Action != "stop" || stop.EarliestStart != 40 ||
		len(stop.DependsOn) != 1 || stop.DependsOn[0] != "activity" {
		t.Fatalf("actions = %#v", plan.LifecycleActions)
	}
	if plan.Predicted.Cost <= 0.01 || plan.Metadata["cloudCost.cloud"] == nil {
		t.Fatalf("prediction = %#v metadata=%#v", plan.Predicted, plan.Metadata)
	}
}

func TestAddCloudLifecycleStopDependsOnAllAssignmentsWithoutPredictedTimes(t *testing.T) {
	resource := planningResource("cloud", 2, 1, 0.001)
	resource.Type = domain.ResourceCloudVM
	plan := domain.SchedulePlan{ID: "plan", Assignments: []domain.PlanAssignment{
		{ID: "first", ResourceID: resource.ID},
		{ID: "second", ResourceID: resource.ID},
	}}
	addCloudLifecycle(&plan, domain.PlanningRequest{Resources: []domain.Resource{resource}})
	stop := plan.LifecycleActions[3]
	if stop.Action != "stop" || len(stop.DependsOn) != 2 || stop.DependsOn[0] != "first" || stop.DependsOn[1] != "second" {
		t.Fatalf("stop action=%+v", stop)
	}
}
