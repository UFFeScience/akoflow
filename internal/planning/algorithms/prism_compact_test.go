package algorithms

import (
	"math"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestCompactPRISMTransferDoesNotDoubleCountSameFlow(t *testing.T) {
	topology := domain.NetworkTopology{Links: []domain.NetworkLink{{
		SourceResourceID:       "source",
		TargetResourceID:       "target",
		BandwidthBitsPerSecond: 80,
		LatencySeconds:         0.25,
	}}}
	state := compactPRISMAddNetworkFlow(
		compactPRISMState{},
		"source",
		"target",
		0,
		3,
	)

	actual := compactPRISMTransferSeconds(
		topology,
		state,
		"source",
		"target",
		10,
		1,
	)
	if math.Abs(actual-2.25) > 1e-9 {
		t.Fatalf("expected one existing flow to produce concurrency 2, got %.2f seconds", actual)
	}
}

func TestCompactPRISMTransferUsesMultiHopPath(t *testing.T) {
	topology := domain.NetworkTopology{Links: []domain.NetworkLink{
		{ID: "a-b", SourceResourceID: "a", TargetResourceID: "b", BandwidthBitsPerSecond: 80, LatencySeconds: 1},
		{ID: "b-c", SourceResourceID: "b", TargetResourceID: "c", BandwidthBitsPerSecond: 40, LatencySeconds: 2},
	}}

	actual := compactPRISMTransferSeconds(topology, compactPRISMState{}, "a", "c", 10, 0)
	if math.Abs(actual-6) > 1e-9 {
		t.Fatalf("expected routed transfer to take 6 seconds, got %.2f", actual)
	}
}

func TestCompactPRISMCostUsesResourceActiveWindow(t *testing.T) {
	request := domain.PlanningRequest{
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{
			planningActivity("a", 10), planningActivity("b", 10),
		}},
		ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}},
		Resources:      []domain.Resource{planningResource("machine", 2, 1, 2)},
	}
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatalf("build compact context: %v", err)
	}
	state := initialCompactPRISMState(search)
	state = compactPRISMPlace(search, state, search.activityOrdinal["a"], 0, 0, true)
	state = compactPRISMPlace(search, state, search.activityOrdinal["b"], 0, 1, true)
	if math.Abs(state.cost-20) > 1e-9 {
		t.Fatalf("expected a 10 second active window at $2/s, got %.2f", state.cost)
	}
}

func TestCompactPRISMCostIncludesFrozenOverheads(t *testing.T) {
	request := domain.PlanningRequest{
		Workflow:       domain.WorkflowVersion{Activities: []domain.Activity{planningActivity("a", 10)}},
		ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}},
		Resources:      []domain.Resource{planningResource("machine", 1, 1, 2)},
	}
	request.Resources[0].BootOverheadSeconds = 3
	request.Resources[0].ContainerOverhead = 1
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatalf("build compact context: %v", err)
	}
	state := compactPRISMPlace(
		search,
		initialCompactPRISMState(search),
		search.activityOrdinal["a"],
		0,
		0,
		true,
	)
	if math.Abs(state.cost-28) > 1e-9 {
		t.Fatalf("expected a 14 second active window at $2/s, got %.2f", state.cost)
	}
	assignment := compactPRISMAssignments(state)[0]
	if prismMetadataNumber(assignment.Metadata, "bootOverheadSeconds") != 3 ||
		prismMetadataNumber(assignment.Metadata, "containerOverheadSeconds") != 1 {
		t.Fatalf("expected frozen overhead metadata, got %#v", assignment.Metadata)
	}
}

func TestDetailedPRISMEvaluatorRedistributesSharedLinkBandwidth(t *testing.T) {
	request := domain.PlanningRequest{
		Workflow: domain.WorkflowVersion{
			Activities: []domain.Activity{
				planningActivity("producer", 1),
				planningActivity("left", 1),
				planningActivity("right", 1),
			},
			Dependencies: []domain.ActivityDependency{
				{ActivityID: "left", DependsOnActivityID: "producer"},
				{ActivityID: "right", DependsOnActivityID: "producer"},
			},
			DataDependencies: []domain.ActivityDataDependency{
				{ProducerActivityID: "producer", ConsumerActivityID: "left", SizeBytes: 10},
				{ProducerActivityID: "producer", ConsumerActivityID: "right", SizeBytes: 10},
			},
		},
		ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}},
		Resources: []domain.Resource{
			planningResource("source", 1, 1, 0),
			planningResource("target", 2, 1, 0),
		},
		NetworkTopology: domain.NetworkTopology{Links: []domain.NetworkLink{{
			ID: "shared", SourceResourceID: "source", TargetResourceID: "target",
			BandwidthBitsPerSecond: 80, SharingPolicy: "SHARED",
		}}},
	}
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatalf("build compact context: %v", err)
	}
	assignments := []domain.PlanAssignment{
		{ActivityID: "producer", ResourceID: "source", CoreID: "source-core-1", PredictedRuntimeSeconds: 1},
		{ActivityID: "left", ResourceID: "target", CoreID: "target-core-1", PredictedRuntimeSeconds: 1},
		{ActivityID: "right", ResourceID: "target", CoreID: "target-core-2", PredictedRuntimeSeconds: 1},
	}
	state := compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0)
	evaluated, err := evaluateCompleteCompactPRISMState(search, state)
	if err != nil {
		t.Fatalf("evaluate candidate: %v", err)
	}
	actual := compactPRISMAssignments(evaluated)
	if math.Abs(evaluated.makespan-4) > 1e-9 {
		t.Fatalf("expected makespan 4 with two 2-second shared transfers, got %.2f", evaluated.makespan)
	}
	if math.Abs(actual[1].PredictedTransferSeconds-2) > 1e-9 ||
		math.Abs(actual[2].PredictedTransferSeconds-2) > 1e-9 {
		t.Fatalf("expected both transfers to receive half bandwidth, got %.2f and %.2f",
			actual[1].PredictedTransferSeconds, actual[2].PredictedTransferSeconds)
	}
}

func TestCompactPRISMReadyFrontierWaitsForEveryPredecessor(t *testing.T) {
	request := domain.PlanningRequest{Workflow: domain.WorkflowVersion{
		Activities: []domain.Activity{
			planningActivity("left", 1),
			planningActivity("right", 1),
			planningActivity("join", 1),
		},
		Dependencies: []domain.ActivityDependency{
			{ActivityID: "join", DependsOnActivityID: "left"},
			{ActivityID: "join", DependsOnActivityID: "right"},
		},
	}, ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}}, Resources: []domain.Resource{
		planningResource("machine", 1, 1, 0),
	}}
	search, err := newCompactPRISMContext(request, map[string]any{"readyBranchLimit": 3})
	if err != nil {
		t.Fatalf("build compact context: %v", err)
	}
	state := initialCompactPRISMState(search)
	join := search.activityOrdinal["join"]
	left := search.activityOrdinal["left"]
	right := search.activityOrdinal["right"]

	state = compactPRISMAdvanceReady(search, state, left)
	if compactPRISMBitIsSet(state.readyTaskBits, join) {
		t.Fatal("join became ready before every predecessor was scheduled")
	}
	state = compactPRISMAdvanceReady(search, state, right)
	if !compactPRISMBitIsSet(state.readyTaskBits, join) {
		t.Fatal("join did not become ready after every predecessor was scheduled")
	}
}

func compactPRISMBitIsSet(words []uint64, ordinal int) bool {
	return words[ordinal/64]&(uint64(1)<<(ordinal%64)) != 0
}
