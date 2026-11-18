package algorithms

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDurationUsesActivityHistoryOnlyForItsResource(t *testing.T) {
	profiles := []domain.ActivityResourceProfile{
		{ActivityTypeID: "same-type", ResourceID: "r", RuntimeSeconds: 20},
		{ActivityID: "observed", ResourceID: "r", RuntimeSeconds: 8},
	}
	activity := domain.Activity{ID: "observed", ActivityTypeID: "same-type", Simulation: &domain.ActivitySimulation{DurationSeconds: 10}}
	if got := duration(activity, domain.Resource{ID: "r"}, profiles); got != 8 {
		t.Fatalf("matched history = %v", got)
	}
	if got := duration(activity, domain.Resource{ID: "other"}, profiles); got != 10 {
		t.Fatalf("different resource = %v", got)
	}
	activity.ID = "same-family"
	if got := duration(activity, domain.Resource{ID: "r"}, profiles); got != 20 {
		t.Fatalf("different activity with same type = %v", got)
	}
}

func TestHEFTUsesHistoricalPreparationWithParallelDependencies(t *testing.T) {
	request := domain.PlanningRequest{
		Workflow: domain.WorkflowVersion{
			ID: "w", Activities: []domain.Activity{{ID: "a"}, {ID: "b"}, {ID: "after"}},
			Dependencies: []domain.ActivityDependency{
				{ActivityID: "after", DependsOnActivityID: "a"},
				{ActivityID: "after", DependsOnActivityID: "b"},
			},
		},
		Resources:      []domain.Resource{{ID: "r", EnvironmentVersionID: "env", CPUCores: 2, CPUCapacity: 2, Schedulable: true, ContainerOverhead: 1}},
		ExecutionScope: domain.ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"env"}},
	}
	for _, id := range []string{"a", "b"} {
		request.ActivityProfiles = append(request.ActivityProfiles, domain.ActivityResourceProfile{
			ActivityID: id, ResourceID: "r", Source: "hub-history", RuntimeSeconds: 5,
			Metadata: map[string]any{"preparationMedianSeconds": 3.0},
		})
	}
	sink := &testSink{}
	if err := (HEFT{}).Schedule(context.Background(), request, nil, nil, sink); err != nil {
		t.Fatal(err)
	}
	plan := sink.plans[0]
	if plan.Predicted.MakespanSeconds != 10 {
		t.Fatalf("want two parallel 8s activities followed by a 2s dependent activity, got %+v", plan.Assignments)
	}
	for _, assignment := range plan.Assignments {
		if assignment.ActivityID != "after" && (assignment.PredictedStartAt != 3 || assignment.PredictedRuntimeSeconds != 5) {
			t.Fatalf("historical preparation and runtime should be separate: %+v", assignment)
		}
	}
}
