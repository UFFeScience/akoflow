package algorithms

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type testSink struct {
	plans []domain.SchedulePlan
}

func (s *testSink) Emit(_ context.Context, plan domain.SchedulePlan) error {
	s.plans = append(s.plans, plan)
	return nil
}

func TestHEFTProducesOneCompletePlan(t *testing.T) {
	request := planningRequestFixture()
	sink := &testSink{}

	if err := (HEFT{}).Schedule(context.Background(), request, nil, nil, sink); err != nil {
		t.Fatalf("schedule HEFT: %v", err)
	}
	if len(sink.plans) != 1 {
		t.Fatalf("expected one HEFT plan, got %d", len(sink.plans))
	}
	assertCompletePlan(t, sink.plans[0], request)
}

func TestPRISMPreservesMultipleOptionsForEachExclusiveObjective(t *testing.T) {
	request := planningRequestFixture()
	for _, scheduler := range []PRISM{NewPRISMTime(), NewPRISMCost()} {
		t.Run(scheduler.Objective, func(t *testing.T) {
			sink := &testSink{}
			configuration := map[string]any{"beamWidth": 20, "optionCount": 6}
			if err := scheduler.Schedule(
				context.Background(),
				request,
				configuration,
				nil,
				sink,
			); err != nil {
				t.Fatalf("schedule PRISM %s: %v", scheduler.Objective, err)
			}
			if len(sink.plans) < 2 {
				t.Fatalf("expected multiple PRISM options, got %d", len(sink.plans))
			}
			for _, plan := range sink.plans {
				assertCompletePlan(t, plan, request)
				if plan.Objective != scheduler.Objective {
					t.Fatalf("expected objective %q, got %q", scheduler.Objective, plan.Objective)
				}
			}
		})
	}
}

func assertCompletePlan(
	t *testing.T,
	plan domain.SchedulePlan,
	request domain.PlanningRequest,
) {
	t.Helper()
	if len(plan.Assignments) != len(request.Workflow.Activities) {
		t.Fatalf("expected %d assignments, got %d", len(request.Workflow.Activities), len(plan.Assignments))
	}
	seen := map[string]bool{}
	for _, assignment := range plan.Assignments {
		seen[assignment.ActivityID] = true
		if assignment.ResourceID == "" || assignment.CoreID == "" {
			t.Fatalf("activity %q has no resource/core", assignment.ActivityID)
		}
	}
	for _, activity := range request.Workflow.Activities {
		if !seen[activity.ID] {
			t.Fatalf("activity %q was not assigned", activity.ID)
		}
	}
}

func planningRequestFixture() domain.PlanningRequest {
	activities := []domain.Activity{
		planningActivity("t1", 10),
		planningActivity("t2", 10),
		planningActivity("t3", 10),
		planningActivity("t4", 10),
		planningActivity("t5", 10),
	}
	dependencies := []domain.ActivityDependency{
		{ActivityID: "t2", DependsOnActivityID: "t1"},
		{ActivityID: "t3", DependsOnActivityID: "t1"},
		{ActivityID: "t4", DependsOnActivityID: "t1"},
		{ActivityID: "t5", DependsOnActivityID: "t2"},
		{ActivityID: "t5", DependsOnActivityID: "t3"},
		{ActivityID: "t5", DependsOnActivityID: "t4"},
	}
	return domain.PlanningRequest{
		Workflow: domain.WorkflowVersion{
			ID:           "workflow-version",
			Activities:   activities,
			Dependencies: dependencies,
		},
		ExecutionScope: domain.ExecutionScope{
			ID:                    "scope",
			EnvironmentVersionIDs: []string{"environment"},
		},
		Resources: []domain.Resource{
			planningResource("m1", 1, 1, 0.3),
			planningResource("m2", 3, 1.5, 0.7),
			planningResource("m3", 1, 2, 1.2),
		},
		NetworkTopology: domain.NetworkTopology{ID: "topology"},
	}
}

func planningActivity(id string, seconds float64) domain.Activity {
	return domain.Activity{
		ID:       id,
		Name:     id,
		Kind:     domain.ActivityKindTask,
		Priority: 1,
		Resources: domain.ActivityResources{
			CPU:         1,
			MemoryBytes: 1,
		},
		Simulation: &domain.ActivitySimulation{DurationSeconds: seconds},
	}
}

func planningResource(
	id string,
	cores int,
	speedup float64,
	price float64,
) domain.Resource {
	return domain.Resource{
		ID:                   id,
		EnvironmentVersionID: "environment",
		ExecutionTarget:      domain.ExecutionTargetDirect,
		CPUCores:             cores,
		CPUCapacity:          float64(cores),
		MemoryBytes:          1024,
		ComputeSpeedup:       speedup,
		PricePerSecond:       price,
		Schedulable:          true,
	}
}
