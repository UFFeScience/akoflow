package algorithms

import (
	"fmt"
	"math"
	"math/bits"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type compactPRISMState struct {
	assignmentTrace    *compactPRISMAssignmentTrace
	assignmentIndex    *compactPRISMAssignmentNode
	coreAvailable      []*compactPRISMCoreNode
	coreOrder          *compactPRISMIntNode
	networkIntervals   *compactPRISMIntervalIndexNode
	resourceActiveFrom []float64
	resourceActiveTo   []float64
	pendingPredChunks  [][]uint16
	readyTaskBits      []uint64
	makespan           float64
	cost               float64
	remainingMinCost   float64
	projectedMakespan  float64
	projectedCost      float64
	signature          uint64
	networkSequence    uint64
}

type compactPRISMTransferResult struct {
	state   compactPRISMState
	readyAt float64
	seconds float64
	cost    float64
}

func initialCompactPRISMState(search compactPRISMContext) compactPRISMState {
	chunkCount := (len(search.activities) + prismPendingChunkSize - 1) / prismPendingChunkSize
	state := compactPRISMState{
		coreAvailable:      compactPRISMInitialCoreRoots(search),
		pendingPredChunks:  make([][]uint16, chunkCount),
		readyTaskBits:      make([]uint64, (len(search.activities)+63)/64),
		signature:          14695981039346656037,
		resourceActiveFrom: make([]float64, len(search.resources)),
		resourceActiveTo:   make([]float64, len(search.resources)),
	}
	for index := range state.resourceActiveFrom {
		state.resourceActiveFrom[index] = math.Inf(1)
	}
	for index := range state.pendingPredChunks {
		state.pendingPredChunks[index] = make([]uint16, prismPendingChunkSize)
	}
	for ordinal, predecessors := range search.predecessors {
		pending := len(predecessors)
		if pending == 0 {
			state.readyTaskBits[ordinal/64] |= uint64(1) << (ordinal % 64)
		} else {
			state.pendingPredChunks[ordinal/prismPendingChunkSize][ordinal%prismPendingChunkSize] = uint16(pending)
		}
		state.remainingMinCost += search.minimumCosts[ordinal]
	}
	// Resource billing is based on the union of active windows, so summing the
	// cheapest per-task costs is not an admissible lower bound: parallel tasks
	// may share the same billed window. The accumulated active-window cost is.
	state.projectedCost = 0
	state.projectedMakespan = compactPRISMReadyRankBound(search, state)
	return state
}

func compactPRISMReadyOrdinals(
	search compactPRISMContext,
	state compactPRISMState,
) []int {
	ready := make([]int, 0, search.readyBranchLimit)
	for wordIndex, word := range state.readyTaskBits {
		for word != 0 && len(ready) < search.readyBranchLimit {
			bit := bits.TrailingZeros64(word)
			ordinal := wordIndex*64 + bit
			if ordinal < len(search.activities) {
				ready = append(ready, ordinal)
			}
			word &^= uint64(1) << bit
		}
		if len(ready) == search.readyBranchLimit {
			break
		}
	}
	return ready
}

func compactPRISMAdvanceReady(
	search compactPRISMContext,
	state compactPRISMState,
	scheduled int,
) compactPRISMState {
	readyBits := append([]uint64(nil), state.readyTaskBits...)
	readyBits[scheduled/64] &^= uint64(1) << (scheduled % 64)
	pendingChunks := append([][]uint16(nil), state.pendingPredChunks...)
	clonedChunks := make(map[int]bool, len(search.successors[scheduled]))
	for _, successor := range search.successors[scheduled] {
		chunkIndex := successor / prismPendingChunkSize
		offset := successor % prismPendingChunkSize
		pending := pendingChunks[chunkIndex][offset]
		if pending == 0 {
			continue
		}
		if !clonedChunks[chunkIndex] {
			pendingChunks[chunkIndex] = append([]uint16(nil), pendingChunks[chunkIndex]...)
			clonedChunks[chunkIndex] = true
		}
		pending--
		pendingChunks[chunkIndex][offset] = pending
		if pending == 0 {
			readyBits[successor/64] |= uint64(1) << (successor % 64)
		}
	}
	state.readyTaskBits = readyBits
	state.pendingPredChunks = pendingChunks
	return state
}

func compactPRISMPlace(
	search compactPRISMContext,
	state compactPRISMState,
	activityOrdinal int,
	resourceOrdinal int,
	sequence int,
	contention bool,
) compactPRISMState {
	resource := search.resources[resourceOrdinal]
	coreRoot := state.coreAvailable[resourceOrdinal]
	coreOrdinal := coreRoot.bestKey
	coreID := resource.cores[coreOrdinal-resource.coreOffset]
	transfer := compactPRISMResolveTransfers(
		search,
		state,
		activityOrdinal,
		resource.resource.ID,
		contention,
	)
	start := math.Max(transfer.readyAt, coreRoot.bestValue)
	order := compactPRISMIntLookup(state.coreOrder, coreOrdinal)
	boot := 0.0
	if order == 0 {
		boot = resource.resource.BootOverheadSeconds
		start += boot
	}
	container := resource.resource.ContainerOverhead
	start += container
	runtime := search.durations[activityOrdinal][resourceOrdinal]
	assignment := compactPRISMBuildAssignment(
		search.activities[activityOrdinal],
		resource.resource,
		coreID,
		sequence,
		order,
		transfer,
		start,
		runtime,
		boot,
		container,
	)
	return compactPRISMCommitPlacement(
		search,
		state,
		transfer.state,
		activityOrdinal,
		resourceOrdinal,
		coreOrdinal,
		sequence,
		assignment,
	)
}

func compactPRISMResolveTransfers(
	search compactPRISMContext,
	state compactPRISMState,
	activityOrdinal int,
	resourceID string,
	contention bool,
) compactPRISMTransferResult {
	result := compactPRISMTransferResult{state: state}
	for _, dependency := range search.predecessors[activityOrdinal] {
		predecessor, exists := compactPRISMAssignmentLookup(
			state.assignmentIndex,
			dependency.predecessor,
		)
		if !exists {
			continue
		}
		flowReadyAt := predecessor.PredictedFinishAt
		transferState := compactPRISMState{}
		if contention {
			transferState = result.state
		}
		transfer := compactPRISMTransferSeconds(
			search.request.NetworkTopology,
			transferState,
			predecessor.ResourceID,
			resourceID,
			dependency.bytes,
			flowReadyAt,
		)
		result.seconds += transfer
		route, routeExists := compactPRISMShortestRoute(
			search.request.NetworkTopology,
			predecessor.ResourceID,
			resourceID,
			dependency.bytes,
		)
		if predecessor.ResourceID != resourceID && dependency.bytes > 0 && routeExists {
			for _, hop := range route.hops {
				result.cost += float64(dependency.bytes) * hop.link.PricePerByte
			}
		}
		deliveredAt := flowReadyAt + transfer
		result.readyAt = math.Max(result.readyAt, deliveredAt)
		if contention && predecessor.ResourceID != resourceID && dependency.bytes > 0 {
			result.state = compactPRISMAddNetworkFlowOnRoute(
				result.state,
				predecessor.ResourceID,
				resourceID,
				flowReadyAt,
				deliveredAt,
				route.hops,
			)
		}
	}
	return result
}

func compactPRISMBuildAssignment(
	activity domain.Activity,
	resource domain.Resource,
	coreID string,
	sequence int,
	order int,
	transfer compactPRISMTransferResult,
	start float64,
	runtime float64,
	boot float64,
	container float64,
) domain.PlanAssignment {
	return domain.PlanAssignment{
		ID: fmt.Sprintf(
			"assignment-%d-%s-%s",
			sequence,
			activity.ID,
			resource.ID,
		),
		ActivityID: activity.ID, ResourceID: resource.ID,
		CoreID: coreID, OrderOnResource: order, Priority: activity.Priority,
		PredictedReadyAt: transfer.readyAt, PredictedStartAt: start,
		PredictedFinishAt: start + runtime, PredictedRuntimeSeconds: runtime,
		PredictedTransferSeconds: transfer.seconds,
		PredictedCost:            runtime * resource.PricePerSecond,
		Metadata: map[string]any{
			"scheduleBasis":            "algorithm",
			"expectedDurationSeconds":  runtime,
			"networkContentionModel":   "known-active-flows",
			"bootOverheadSeconds":      boot,
			"containerOverheadSeconds": container,
			"queueSeconds":             math.Max(0, start-transfer.readyAt-boot-container),
			"transferCost":             transfer.cost,
		},
	}
}

func compactPRISMCommitPlacement(
	search compactPRISMContext,
	previous compactPRISMState,
	out compactPRISMState,
	activityOrdinal int,
	resourceOrdinal int,
	coreOrdinal int,
	sequence int,
	assignment domain.PlanAssignment,
) compactPRISMState {
	activity := search.activities[activityOrdinal]
	out.resourceActiveFrom = append([]float64(nil), previous.resourceActiveFrom...)
	out.resourceActiveTo = append([]float64(nil), previous.resourceActiveTo...)
	previousWindow := 0.0
	if !math.IsInf(out.resourceActiveFrom[resourceOrdinal], 1) {
		previousWindow = out.resourceActiveTo[resourceOrdinal] - out.resourceActiveFrom[resourceOrdinal]
	}
	billingStart := assignment.PredictedStartAt -
		prismMetadataNumber(assignment.Metadata, "bootOverheadSeconds") -
		prismMetadataNumber(assignment.Metadata, "containerOverheadSeconds")
	out.resourceActiveFrom[resourceOrdinal] = math.Min(
		out.resourceActiveFrom[resourceOrdinal],
		billingStart,
	)
	out.resourceActiveTo[resourceOrdinal] = math.Max(
		out.resourceActiveTo[resourceOrdinal],
		assignment.PredictedFinishAt,
	)
	activeWindow := out.resourceActiveTo[resourceOrdinal] - out.resourceActiveFrom[resourceOrdinal]
	assignment.PredictedCost = (activeWindow-previousWindow)*search.resources[resourceOrdinal].resource.PricePerSecond + prismMetadataNumber(assignment.Metadata, "transferCost")
	out.cost += assignment.PredictedCost
	out.assignmentTrace = &compactPRISMAssignmentTrace{
		assignment: assignment,
		previous:   previous.assignmentTrace,
		length:     sequence + 1,
	}
	out.assignmentIndex = compactPRISMAssignmentInsert(
		previous.assignmentIndex,
		activityOrdinal,
		assignment,
	)
	out.coreAvailable = append([]*compactPRISMCoreNode(nil), previous.coreAvailable...)
	out.coreAvailable[resourceOrdinal] = compactPRISMCoreInsert(
		previous.coreAvailable[resourceOrdinal],
		coreOrdinal,
		assignment.PredictedFinishAt,
	)
	out.coreOrder = compactPRISMIntInsert(previous.coreOrder, coreOrdinal, assignment.OrderOnResource+1)
	out.makespan = math.Max(previous.makespan, assignment.PredictedFinishAt)
	out.remainingMinCost -= search.minimumCosts[activityOrdinal]
	out.signature = extendScheduleSignature(
		previous.signature,
		activity.ID,
		assignment.ResourceID,
		assignment.CoreID,
	)
	out = compactPRISMAdvanceReady(search, out, activityOrdinal)
	out.projectedCost = out.cost
	out.projectedMakespan = math.Max(
		out.makespan,
		compactPRISMReadyRankBound(search, out),
	)
	return out
}

func compactPRISMReadyRankBound(
	search compactPRISMContext,
	state compactPRISMState,
) float64 {
	bound := 0.0
	for wordIndex, word := range state.readyTaskBits {
		for word != 0 {
			bit := bits.TrailingZeros64(word)
			ordinal := wordIndex*64 + bit
			if ordinal < len(search.ranks) {
				bound = math.Max(bound, search.ranks[ordinal])
			}
			word &^= uint64(1) << bit
		}
	}
	return bound
}

func compactPRISMAssignments(state compactPRISMState) []domain.PlanAssignment {
	if state.assignmentTrace == nil {
		return nil
	}
	items := make([]domain.PlanAssignment, state.assignmentTrace.length)
	for trace := state.assignmentTrace; trace != nil; trace = trace.previous {
		items[trace.length-1] = trace.assignment
	}
	return items
}
