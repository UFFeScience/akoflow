package algorithms

import (
	"context"
	"reflect"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func slaAblationFixture() domain.PlanningRequest {
	return domain.PlanningRequest{
		Workflow:        domain.WorkflowVersion{ID: "workflow", Activities: []domain.Activity{planningActivity("task", 10)}},
		ExecutionScope:  domain.ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"environment"}},
		Resources:       []domain.Resource{planningResource("fast", 1, 10, 1), planningResource("slow", 1, 1, 0)},
		DeadlineSeconds: 20,
		Budget:          0.5,
	}
}

func TestPRISMSLAAblationChangesOnlyConstraintGuidance(t *testing.T) {
	r := slaAblationFixture()
	on, off := &testSink{}, &testSink{}
	if err := NewPRISMTime().Schedule(context.Background(), r, map[string]any{"slaAware": true}, nil, on); err != nil {
		t.Fatal(err)
	}
	if err := NewPRISMTime().Schedule(context.Background(), r, map[string]any{"slaAware": false}, nil, off); err != nil {
		t.Fatal(err)
	}
	if on.plans[0].Assignments[0].ResourceID != "slow" || off.plans[0].Assignments[0].ResourceID != "fast" {
		t.Fatal("fixture must distinguish budget-guided selection from unconstrained speed")
	}
	if !on.plans[0].Predicted.Feasible || off.plans[0].Predicted.Feasible {
		t.Fatal("original SLA must still determine emitted predicted feasibility")
	}
	if off.plans[0].Budget != r.Budget || off.plans[0].DeadlineSeconds != r.DeadlineSeconds || off.plans[0].Metadata["slaAware"] != false {
		t.Fatal("ablation erased limits or provenance")
	}
}

func TestPRISMSLAOffReproducesUnconstrainedSearchForBothObjectives(t *testing.T) {
	for _, scheduler := range []PRISM{NewPRISMTime(), NewPRISMCost()} {
		t.Run(scheduler.Objective, func(t *testing.T) {
			r := slaAblationFixture()
			r.DeadlineSeconds, r.Budget = 0.01, 0.0001
			unconstrained := r
			unconstrained.DeadlineSeconds, unconstrained.Budget = 0, 0
			a, b := &testSink{}, &testSink{}
			configuration := map[string]any{"slaAware": false, "beamWidth": 20, "optionCount": 25}
			if err := scheduler.Schedule(context.Background(), r, configuration, nil, a); err != nil {
				t.Fatal(err)
			}
			if err := scheduler.Schedule(context.Background(), unconstrained, configuration, nil, b); err != nil {
				t.Fatal(err)
			}
			if len(a.plans) != len(b.plans) {
				t.Fatal("limits changed the ablated candidate set")
			}
			for i := range a.plans {
				if !reflect.DeepEqual(a.plans[i].Assignments, b.plans[i].Assignments) || a.plans[i].Predicted.MakespanSeconds != b.plans[i].Predicted.MakespanSeconds || a.plans[i].Predicted.Cost != b.plans[i].Predicted.Cost {
					t.Fatal("ablation changed unconstrained scheduling decisions or forecast")
				}
				if a.plans[i].Predicted.Feasible {
					t.Fatal("infeasible original deadline was silently removed")
				}
			}
		})
	}
}

func TestPRISMTiedCandidatesRemainDeterministic(t *testing.T) {
	r := slaAblationFixture()
	r.DeadlineSeconds, r.Budget = 0, 0
	var expected []domain.SchedulePlan
	for repeat := 0; repeat < 30; repeat++ {
		out := &testSink{}
		if err := NewPRISMTime().Schedule(context.Background(), r, map[string]any{"beamWidth": 20}, nil, out); err != nil {
			t.Fatal(err)
		}
		if repeat == 0 {
			expected = out.plans
		} else if !reflect.DeepEqual(expected, out.plans) {
			t.Fatal("tied native-HEFT/search origins or assignments changed between identical runs")
		}
	}
}

func TestPRISMSLADefaultIsEnabledAndRejectsInvalidToggle(t *testing.T) {
	r := slaAblationFixture()
	a, b := &testSink{}, &testSink{}
	if err := NewPRISMTime().Schedule(context.Background(), r, nil, nil, a); err != nil {
		t.Fatal(err)
	}
	if err := NewPRISMTime().Schedule(context.Background(), r, map[string]any{"slaAware": true}, nil, b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.plans, b.plans) {
		t.Fatal("default behavior differs from SLA-aware behavior")
	}
	if err := NewPRISMTime().Schedule(context.Background(), r, map[string]any{"slaAware": "false"}, nil, &testSink{}); err == nil {
		t.Fatal("invalid ablation configuration accepted")
	}
}
