package algorithms

import (
	"math"
	"sort"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestCompactPRISMAssignmentChunksUseCopyOnWrite(t *testing.T) {
	base := make([][]compactPRISMAssignment, 2)
	first := compactPRISMAssignment{activityOrdinal: 1, resourceOrdinal: 2}
	withFirst := compactPRISMAssignmentChunkInsert(base, 1, first)
	second := compactPRISMAssignment{activityOrdinal: 3, resourceOrdinal: 4}
	withSecond := compactPRISMAssignmentChunkInsert(withFirst, prismPendingChunkSize+2, second)

	if _, exists := compactPRISMAssignmentChunkLookup(base, 1); exists {
		t.Fatal("copy-on-write insert mutated the base assignment table")
	}
	actualFirst, exists := compactPRISMAssignmentChunkLookup(withFirst, 1)
	if !exists || actualFirst.activityOrdinal != first.activityOrdinal || actualFirst.resourceOrdinal != first.resourceOrdinal {
		t.Fatalf("first assignment = %#v, %t; want %#v, true", actualFirst, exists, first)
	}
	if _, exists := compactPRISMAssignmentChunkLookup(withFirst, prismPendingChunkSize+2); exists {
		t.Fatal("copy-on-write insert mutated the previous assignment table")
	}
	actualSecond, exists := compactPRISMAssignmentChunkLookup(withSecond, prismPendingChunkSize+2)
	if !exists || actualSecond.activityOrdinal != second.activityOrdinal || actualSecond.resourceOrdinal != second.resourceOrdinal {
		t.Fatalf("second assignment = %#v, %t; want %#v, true", actualSecond, exists, second)
	}
}

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

func TestCompactPRISMActiveIntervalIndexCountsBoundariesPersistently(t *testing.T) {
	base := compactPRISMState{}
	one := compactPRISMAddNetworkFlow(base, "source", "target", 0, 3)
	two := compactPRISMAddNetworkFlow(one, "source", "target", 1, 4)
	three := compactPRISMAddNetworkFlow(two, "source", "target", 3, 5)

	intervals := func(state compactPRISMState) *compactPRISMIntervalSet {
		return compactPRISMIntervalIndexLookup(state.networkIntervals, "s:source")
	}
	if got := compactPRISMActiveIntervals(intervals(one), 2); got != 1 {
		t.Fatalf("persistent base count = %d, want 1", got)
	}
	for instant, want := range map[float64]int{0: 1, 2: 2, 3: 2, 4: 1, 5: 0} {
		if got := compactPRISMActiveIntervals(intervals(three), instant); got != want {
			t.Fatalf("active intervals at %.0f = %d, want %d", instant, got, want)
		}
	}
}

func TestCompactPRISMNetworkBatchMatchesSequentialFlowIndex(t *testing.T) {
	base := compactPRISMAddNetworkFlow(compactPRISMState{}, "source", "target", 0, 2)
	sequential := compactPRISMAddNetworkFlow(base, "source", "target", 1, 4)
	sequential = compactPRISMAddNetworkFlow(sequential, "source", "target", 3, 6)

	batch := newCompactPRISMNetworkBatch(base)
	batch.add("source", "target", 1, 4, nil)
	batch.add("source", "target", 3, 6, nil)
	batched := batch.commit()

	for _, key := range []string{"s:source", "d:target", "p:source\x00target"} {
		for _, instant := range []float64{0, 1, 2, 3, 4, 6} {
			want := compactPRISMActiveIntervals(
				compactPRISMIntervalIndexLookup(sequential.networkIntervals, key), instant,
			)
			got := compactPRISMActiveIntervals(
				compactPRISMIntervalIndexLookup(batched.networkIntervals, key), instant,
			)
			if got != want {
				t.Fatalf("%s active at %.0f = %d, want %d", key, instant, got, want)
			}
		}
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

func TestCompactPRISMCachedRoutePreservesTransferResult(t *testing.T) {
	topology := domain.NetworkTopology{Links: []domain.NetworkLink{
		{ID: "a-b", SourceResourceID: "a", TargetResourceID: "b", BandwidthBitsPerSecond: 80, LatencySeconds: 1},
		{ID: "b-c", SourceResourceID: "b", TargetResourceID: "c", BandwidthBitsPerSecond: 40, LatencySeconds: 2},
	}}
	resources := []domain.Resource{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	router := newCompactPRISMRouter(topology, resources)
	route, exists := router.route("a", "c")
	if !exists {
		t.Fatal("expected cached a-to-c route")
	}
	want := compactPRISMTransferSeconds(topology, compactPRISMState{}, "a", "c", 10, 0)
	got := compactPRISMTransferSecondsOnRoute(compactPRISMState{}, "a", "c", 10, 0, route)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("cached route changed transfer result: got %.3f, want %.3f", got, want)
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

func TestDetailedPRISMEvaluatorAppliesPairwiseInterferenceOnSameResource(t *testing.T) {
	request := domain.PlanningRequest{
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{
			planningActivity("a", 10), planningActivity("b", 10),
		}},
		ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}},
		Resources:      []domain.Resource{planningResource("machine", 1, 1, 0)},
		Interference: &domain.InterferenceMatrix{
			SchemaVersion: "1", Model: "pairwise-cpu-priority", Aggregation: "minimum",
			Entries: []domain.InterferenceEntry{
				{AffectedActivityID: "a", InterferingActivityID: "b", PriorityWeight: 0.9230769230769231},
				{AffectedActivityID: "b", InterferingActivityID: "a", PriorityWeight: 1.0810810810810811},
			},
		},
	}
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatalf("build compact context: %v", err)
	}
	assignments := []domain.PlanAssignment{
		{ActivityID: "a", ResourceID: "machine", CoreID: "machine-core-1", PredictedRuntimeSeconds: 10},
		{ActivityID: "b", ResourceID: "machine", CoreID: "machine-core-2", PredictedRuntimeSeconds: 10},
	}
	state := compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0)
	evaluated, err := evaluateCompleteCompactPRISMState(search, state)
	if err != nil {
		t.Fatalf("evaluate candidate: %v", err)
	}
	if math.Abs(evaluated.makespan-20) > 1e-6 {
		t.Fatalf("expected priority sharing to conserve 20 seconds of CPU work, got %.2f", evaluated.makespan)
	}
	evaluatedAssignments := compactPRISMAssignments(search, evaluated)
	if math.Abs(evaluatedAssignments[0].PredictedFinishAt-evaluatedAssignments[1].PredictedFinishAt) < 1 {
		t.Fatalf("expected asymmetric priorities to produce different finish times: %#v", evaluatedAssignments)
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
	assignment := compactPRISMAssignments(search, state)[0]
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
	actual := compactPRISMAssignments(search, evaluated)
	if math.Abs(evaluated.makespan-4) > 1e-9 {
		t.Fatalf("expected makespan 4 with two 2-second shared transfers, got %.2f", evaluated.makespan)
	}
	if math.Abs(actual[1].PredictedTransferSeconds-2) > 1e-9 ||
		math.Abs(actual[2].PredictedTransferSeconds-2) > 1e-9 {
		t.Fatalf("expected both transfers to receive half bandwidth, got %.2f and %.2f",
			actual[1].PredictedTransferSeconds, actual[2].PredictedTransferSeconds)
	}
}

func TestDetailedPRISMEvaluatorRedistributesSharedSourceBandwidthAcrossRoutes(t *testing.T) {
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
			planningResource("left-target", 1, 1, 0),
			planningResource("right-target", 1, 1, 0),
		},
		NetworkTopology: domain.NetworkTopology{Links: []domain.NetworkLink{
			{ID: "left-route", SourceResourceID: "source", TargetResourceID: "left-target", BandwidthBitsPerSecond: 80, SharingPolicy: "SHARED"},
			{ID: "right-route", SourceResourceID: "source", TargetResourceID: "right-target", BandwidthBitsPerSecond: 80, SharingPolicy: "SHARED"},
		}},
	}
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatalf("build compact context: %v", err)
	}
	assignments := []domain.PlanAssignment{
		{ActivityID: "producer", ResourceID: "source", CoreID: "source-core-1", PredictedRuntimeSeconds: 1},
		{ActivityID: "left", ResourceID: "left-target", CoreID: "left-target-core-1", PredictedRuntimeSeconds: 1},
		{ActivityID: "right", ResourceID: "right-target", CoreID: "right-target-core-1", PredictedRuntimeSeconds: 1},
	}
	state := compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0)
	evaluated, err := evaluateCompleteCompactPRISMState(search, state)
	if err != nil {
		t.Fatalf("evaluate candidate: %v", err)
	}
	actual := compactPRISMAssignments(search, evaluated)
	if math.Abs(evaluated.makespan-4) > 1e-9 {
		t.Fatalf("expected makespan 4 with the source shared across routes, got %.2f", evaluated.makespan)
	}
	if math.Abs(actual[1].PredictedTransferSeconds-2) > 1e-9 ||
		math.Abs(actual[2].PredictedTransferSeconds-2) > 1e-9 {
		t.Fatalf("expected both transfers to share source bandwidth, got %.2f and %.2f",
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

func TestCompactPRISMReadyRankBoundUsesHighestRankedReadyActivity(t *testing.T) {
	search := compactPRISMContext{ranks: []float64{100, 80, 60, 40}}
	state := compactPRISMState{readyTaskBits: []uint64{
		uint64(1)<<1 | uint64(1)<<3,
	}}
	if got := compactPRISMReadyRankBound(search, state); got != 80 {
		t.Fatalf("expected highest ready rank 80, got %.0f", got)
	}
}

func TestCompactPRISMTimePrefersLessCoreWaitingWhenProjectedMakespanTies(t *testing.T) {
	states := []compactPRISMState{
		{projectedMakespan: 20, projectedCost: 1, queueSeconds: 8, signature: 1},
		{projectedMakespan: 20, projectedCost: 4, queueSeconds: 2, signature: 2},
	}
	got := compactPRISMTopStates(states, 1, "time")
	if len(got) != 1 || got[0].signature != 2 {
		t.Fatalf("expected lower-wait state, got %#v", got)
	}
}

func TestCompactPRISMCostUsesCoreWaitingAfterExclusiveCostAndTime(t *testing.T) {
	states := []compactPRISMState{
		{projectedCost: 2, projectedMakespan: 20, queueSeconds: 8, signature: 1},
		{projectedCost: 2, projectedMakespan: 20, queueSeconds: 2, signature: 2},
	}
	got := compactPRISMTopStates(states, 1, "cost")
	if len(got) != 1 || got[0].signature != 2 {
		t.Fatalf("expected lower-wait equal-cost state, got %#v", got)
	}
}

func TestCompactPRISMEarliestFinishLaneUsesConcretePartialSchedule(t *testing.T) {
	states := []compactPRISMState{
		{makespan: 12, projectedMakespan: 15, queueSeconds: 1, signature: 1},
		{makespan: 10, projectedMakespan: 18, queueSeconds: 3, signature: 2},
	}
	got := compactPRISMTopStates(states, 1, "earliest-finish")
	if len(got) != 1 || got[0].signature != 2 {
		t.Fatalf("expected earliest concrete finish state, got %#v", got)
	}
}

func TestCompactPRISMTimeBreaksTiesByNetworkLocality(t *testing.T) {
	states := []compactPRISMState{
		{projectedMakespan: 20, transferSeconds: 4, networkCost: 1, usedResourceCount: 3, signature: 1},
		{projectedMakespan: 20, transferSeconds: 2, networkCost: 1, usedResourceCount: 2, signature: 2},
	}
	got := compactPRISMTopStates(states, 1, "time")
	if len(got) != 1 || got[0].signature != 2 {
		t.Fatalf("expected the closest equal-time state, got %#v", got)
	}
}

func TestCompactPRISMCostBreaksTiesByNetworkCost(t *testing.T) {
	states := []compactPRISMState{
		{projectedCost: 2, networkCost: 1, transferSeconds: 1, signature: 1},
		{projectedCost: 2, networkCost: 0.5, transferSeconds: 3, signature: 2},
	}
	got := compactPRISMTopStates(states, 1, "cost")
	if len(got) != 1 || got[0].signature != 2 {
		t.Fatalf("expected the lower-network-cost state, got %#v", got)
	}
}

func TestCompactPRISMBeamPreservesConsolidatedLocalityState(t *testing.T) {
	states := make([]compactPRISMState, 20)
	for index := range states {
		states[index] = compactPRISMState{
			projectedMakespan: 10 + float64(index)/10,
			projectedCost:     1 + float64(index)/10,
			transferSeconds:   10,
			networkCost:       2,
			usedResourceCount: 4,
			signature:         uint64(index + 1),
		}
	}
	// This state is deliberately worse for the primary time ordering, but is
	// the only data-local, single-resource path and must survive its beam lane.
	states[19].projectedMakespan = 50
	states[19].projectedCost = 20
	states[19].transferSeconds = 0
	states[19].networkCost = 0
	states[19].usedResourceCount = 1

	got := selectCompactPRISMBeam(states, 10, "time")
	found := false
	for _, state := range got {
		found = found || state.signature == states[19].signature
	}
	if !found {
		t.Fatalf("expected consolidated state to survive the locality lane: %#v", got)
	}
}

func TestCompactPRISMPrunesOnlyConfiguredMonotonicLimits(t *testing.T) {
	states := []compactPRISMState{
		{makespan: 9, cost: 4},
		{makespan: 11, cost: 4},
		{makespan: 9, cost: 6},
	}
	request := domain.PlanningRequest{DeadlineSeconds: 10, Budget: 5}
	got := pruneCompactPRISMStates(states, request)
	if len(got) != 1 || got[0].makespan != 9 || got[0].cost != 4 {
		t.Fatalf("unexpected pruned states: %#v", got)
	}
	if got := pruneCompactPRISMStates(states, domain.PlanningRequest{}); len(got) != len(states) {
		t.Fatalf("unbounded request pruned %d states", len(states)-len(got))
	}
}

func TestCompactPRISMTopStatesMatchesFullSort(t *testing.T) {
	states := make([]compactPRISMState, 250)
	for index := range states {
		states[index] = compactPRISMState{
			projectedMakespan: float64((index * 37) % 97),
			projectedCost:     float64((index * 53) % 89),
			signature:         uint64(index + 1),
		}
	}
	want := append([]compactPRISMState(nil), states...)
	sort.Slice(want, func(i, j int) bool {
		return compactPRISMStateLess(want[i], want[j], "time")
	})
	want = want[:25]
	got := compactPRISMTopStates(states, 25, "time")
	for index := range want {
		if got[index].signature != want[index].signature {
			t.Fatalf(
				"top-k differs at %d: got signature %d, want %d",
				index,
				got[index].signature,
				want[index].signature,
			)
		}
	}
}

func compactPRISMBitIsSet(words []uint64, ordinal int) bool {
	return words[ordinal/64]&(uint64(1)<<(ordinal%64)) != 0
}
