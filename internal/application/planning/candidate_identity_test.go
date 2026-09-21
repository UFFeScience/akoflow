package planning

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestReidentifyCandidatePlanRewritesLifecycleIdentityAndDependencies(t *testing.T) {
	plan := domain.SchedulePlan{
		ID: "temporary-plan",
		Assignments: []domain.PlanAssignment{{
			ID: "temporary-plan-activity", PlanID: "temporary-plan", ActivityID: "activity",
		}},
		LifecycleActions: []domain.PlannedLifecycleAction{
			{
				ID: "temporary-plan-provision", SchedulePlanID: "temporary-plan",
				Action: "provision",
			},
			{
				ID: "temporary-plan-stop", SchedulePlanID: "temporary-plan", Action: "stop",
				DependsOn: []string{"temporary-plan-provision", "temporary-plan-activity"},
			},
		},
	}

	reidentifyCandidatePlan(&plan, "persisted-plan")

	if plan.Assignments[0].ID != "persisted-plan-activity" || plan.Assignments[0].PlanID != "persisted-plan" {
		t.Fatalf("assignment identity was not rewritten: %#v", plan.Assignments[0])
	}
	if plan.LifecycleActions[0].ID != "persisted-plan-lifecycle-1" || plan.LifecycleActions[1].ID != "persisted-plan-lifecycle-2" {
		t.Fatalf("lifecycle identities were not rewritten: %#v", plan.LifecycleActions)
	}
	if plan.LifecycleActions[0].SchedulePlanID != "persisted-plan" || plan.LifecycleActions[1].SchedulePlanID != "persisted-plan" {
		t.Fatalf("lifecycle plan ownership was not rewritten: %#v", plan.LifecycleActions)
	}
	want := []string{"persisted-plan-lifecycle-1", "persisted-plan-activity"}
	for index, dependency := range want {
		if plan.LifecycleActions[1].DependsOn[index] != dependency {
			t.Fatalf("dependency %d: got %q, want %q", index, plan.LifecycleActions[1].DependsOn[index], dependency)
		}
	}
}
