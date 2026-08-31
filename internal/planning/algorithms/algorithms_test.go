package algorithms

import (
	"context"
	"reflect"
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

func TestOptimizedHEFTPreservesCanonicalPlacement(t *testing.T) {
	request := planningRequestFixture()
	want := legacyHEFTPlan(t, request)
	sink := &testSink{}
	if err := (HEFT{}).Schedule(context.Background(), request, nil, nil, sink); err != nil {
		t.Fatalf("schedule optimized HEFT: %v", err)
	}
	if !reflect.DeepEqual(sink.plans[0].Assignments, want.Assignments) {
		t.Fatalf(
			"optimized HEFT changed canonical assignments\nactual: %#v\nexpected: %#v",
			sink.plans[0].Assignments,
			want.Assignments,
		)
	}
	if sink.plans[0].Predicted != want.Predicted {
		t.Fatalf(
			"optimized HEFT changed predicted metrics: actual %#v, expected %#v",
			sink.plans[0].Predicted,
			want.Predicted,
		)
	}
}

func legacyHEFTPlan(t *testing.T, request domain.PlanningRequest) domain.SchedulePlan {
	t.Helper()
	order, err := heftOrder(request)
	if err != nil {
		t.Fatalf("order legacy HEFT activities: %v", err)
	}
	state := initialState()
	for index, activity := range order {
		var selected *scheduledState
		for _, resource := range schedulableResources(request) {
			if !resourceFeasible(activity, resource) {
				continue
			}
			for _, core := range cores(resource) {
				candidate := place(request, state, activity, resource, core, index)
				if selected == nil || candidate.makespan < selected.makespan ||
					(candidate.makespan == selected.makespan && candidate.cost < selected.cost) {
					copy := candidate
					selected = &copy
				}
			}
		}
		if selected == nil {
			t.Fatalf("legacy HEFT found no placement for %q", activity.ID)
		}
		state = *selected
	}
	return planFromState("heft-candidate-1", "heft", "time", request, state)
}

func TestHPCMachineExposesDiscoveredCoresToSchedulers(t *testing.T) {
	resource := planningResource("hpc-node", 36, 1, 0)
	resource.ExecutionTarget = domain.ExecutionTargetBatch
	resource.Type = domain.ResourceHPCMachine

	actual := cores(resource)
	if len(actual) != 36 {
		t.Fatalf("expected 36 schedulable HPC cores, got %d", len(actual))
	}
}

func TestHEFTDistributesParallelActivitiesAcrossHPCMachineCores(t *testing.T) {
	resource := planningResource("hpc-node", 4, 1, 0)
	resource.ExecutionTarget = domain.ExecutionTargetBatch
	resource.Type = domain.ResourceHPCMachine
	request := domain.PlanningRequest{
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{
			planningActivity("t1", 1), planningActivity("t2", 1),
			planningActivity("t3", 1), planningActivity("t4", 1),
		}},
		ExecutionScope: domain.ExecutionScope{
			EnvironmentVersionIDs: []string{"environment"},
		},
		Resources: []domain.Resource{resource},
	}
	sink := &testSink{}
	if err := (HEFT{}).Schedule(context.Background(), request, nil, nil, sink); err != nil {
		t.Fatalf("schedule parallel HPC work: %v", err)
	}
	used := map[string]bool{}
	for _, assignment := range sink.plans[0].Assignments {
		used[assignment.CoreID] = true
	}
	if len(used) != 4 {
		t.Fatalf("expected HEFT to use four HPC cores, got %v", used)
	}
}

func TestAbstractBatchQueueRemainsASingleSchedulerTarget(t *testing.T) {
	resource := planningResource("partition", 36, 1, 0)
	resource.ExecutionTarget = domain.ExecutionTargetBatch
	resource.Type = domain.ResourceHPCPartition

	actual := cores(resource)
	if len(actual) != 1 || actual[0] != "partition-slot" {
		t.Fatalf("expected one opaque partition slot, got %v", actual)
	}
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

func TestPRISMTimeIsEvaluatedIndependentlyFromCanonicalHEFT(t *testing.T) {
	request := planningRequestFixture()
	prismSink := &testSink{}
	if err := NewPRISMTime().Schedule(
		context.Background(),
		request,
		map[string]any{"beamWidth": 20, "optionCount": 6},
		nil,
		prismSink,
	); err != nil {
		t.Fatalf("schedule PRISM Time: %v", err)
	}
	if len(prismSink.plans) == 0 {
		t.Fatal("expected PRISM to return its independently evaluated options")
	}
	assertCompletePlan(t, prismSink.plans[0], request)
}

func TestPRISMReturnsBestEffortAnchorWhenSLAPrunesEverySearchState(t *testing.T) {
	request := planningRequestFixture()
	request.DeadlineSeconds = 0.001
	request.Budget = 0.001
	sink := &testSink{}
	if err := NewPRISMTime().Schedule(
		context.Background(),
		request,
		map[string]any{"beamWidth": 20, "optionCount": 6},
		nil,
		sink,
	); err != nil {
		t.Fatalf("schedule constrained PRISM: %v", err)
	}
	if len(sink.plans) == 0 {
		t.Fatal("expected PRISM best-effort plans")
	}
	for _, plan := range sink.plans {
		if plan.Predicted.Feasible {
			t.Fatal("expected best-effort plan to be marked infeasible")
		}
		assertCompletePlan(t, plan, request)
	}
}

func TestPRISMPriorityRankIncludesCommunication(t *testing.T) {
	request := planningRequestFixture()
	request.Workflow.DataDependencies = []domain.ActivityDataDependency{
		{ProducerActivityID: "t1", ConsumerActivityID: "t2", SizeBytes: 1_000_000_000},
	}
	request.NetworkTopology.Links = []domain.NetworkLink{
		{
			SourceResourceID:       "m1",
			TargetResourceID:       "m2",
			BandwidthBitsPerSecond: 8_000_000_000,
			Bidirectional:          true,
		},
	}
	order, err := topologicalOrder(request.Workflow)
	if err != nil {
		t.Fatalf("topological order: %v", err)
	}
	ranks, err := prismCommunicationRanks(request, order, schedulableResources(request))
	if err != nil {
		t.Fatalf("PRISM ranks: %v", err)
	}
	if ranks["t1"] <= ranks["t2"] {
		t.Fatalf("expected predecessor rank %.3f to include communication and exceed successor %.3f", ranks["t1"], ranks["t2"])
	}
}

func TestPRISMTransferCountsKnownActiveFlowAtSharedSource(t *testing.T) {
	flow := scheduledNetworkFlow{
		sourceResourceID:      "source",
		destinationResourceID: "another-target",
		readyAt:               0,
		deliveredAt:           3,
	}
	assertPRISMTransferSeconds(t, flow, 2.25)
}

func TestPRISMTransferCountsKnownActiveFlowAtSharedDestination(t *testing.T) {
	flow := scheduledNetworkFlow{
		sourceResourceID:      "another-source",
		destinationResourceID: "target",
		readyAt:               0,
		deliveredAt:           3,
	}
	assertPRISMTransferSeconds(t, flow, 2.25)
}

func TestPRISMTransferIgnoresDisjointActiveFlow(t *testing.T) {
	flow := scheduledNetworkFlow{
		sourceResourceID:      "another-source",
		destinationResourceID: "another-target",
		readyAt:               0,
		deliveredAt:           3,
	}
	assertPRISMTransferSeconds(t, flow, 1.25)
}

func TestPRISMTransferIgnoresFlowDeliveredAtReadyTime(t *testing.T) {
	flow := scheduledNetworkFlow{
		sourceResourceID:      "source",
		destinationResourceID: "another-target",
		readyAt:               0,
		deliveredAt:           1,
	}
	assertPRISMTransferSeconds(t, flow, 1.25)
}

func assertPRISMTransferSeconds(t *testing.T, flow scheduledNetworkFlow, expected float64) {
	t.Helper()
	topology := domain.NetworkTopology{
		Links: []domain.NetworkLink{
			{
				SourceResourceID:       "source",
				TargetResourceID:       "target",
				BandwidthBitsPerSecond: 80,
				LatencySeconds:         0.25,
			},
		},
	}
	actual := prismTransferSeconds(topology, "source", "target", 10, 1, []scheduledNetworkFlow{flow})
	if actual != expected {
		t.Fatalf("expected %.2f seconds, got %.2f", expected, actual)
	}
}

func TestHEFTPlacementIgnoresPRISMNetworkFlowState(t *testing.T) {
	request := domain.PlanningRequest{
		Workflow: domain.WorkflowVersion{
			Activities: []domain.Activity{
				planningActivity("producer", 1),
				planningActivity("consumer", 1),
			},
			Dependencies: []domain.ActivityDependency{
				{ActivityID: "consumer", DependsOnActivityID: "producer"},
			},
			DataDependencies: []domain.ActivityDataDependency{
				{ProducerActivityID: "producer", ConsumerActivityID: "consumer", SizeBytes: 10},
			},
		},
		NetworkTopology: domain.NetworkTopology{
			Links: []domain.NetworkLink{
				{
					SourceResourceID:       "source",
					TargetResourceID:       "target",
					BandwidthBitsPerSecond: 80,
				},
			},
		},
	}
	state := initialState()
	state.byActivity["producer"] = domain.PlanAssignment{
		ActivityID:        "producer",
		ResourceID:        "source",
		PredictedFinishAt: 1,
	}
	state.networkFlows = []scheduledNetworkFlow{
		{
			sourceResourceID:      "source",
			destinationResourceID: "another-target",
			readyAt:               0,
			deliveredAt:           3,
		},
	}

	resource := planningResource("target", 1, 1, 0)
	heftState := place(request, state, request.Workflow.Activities[1], resource, "target-core-1", 1)
	prismState := placePRISM(request, state, request.Workflow.Activities[1], resource, "target-core-1", 1)

	if heftState.assignments[0].PredictedTransferSeconds != 1 {
		t.Fatalf(
			"expected HEFT transfer to remain 1 second, got %.2f",
			heftState.assignments[0].PredictedTransferSeconds,
		)
	}
	if prismState.assignments[0].PredictedTransferSeconds != 2 {
		t.Fatalf(
			"expected PRISM transfer to account for contention, got %.2f",
			prismState.assignments[0].PredictedTransferSeconds,
		)
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
