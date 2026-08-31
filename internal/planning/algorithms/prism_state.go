package algorithms

import (
	"math"
	"math/bits"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type compactPRISMState struct {
	assignmentTrace      *compactPRISMAssignmentTrace
	assignmentChunks     [][]compactPRISMAssignment
	evaluatedAssignments []domain.PlanAssignment
	coreAvailable        []*compactPRISMCoreNode
	coreOrder            *compactPRISMIntNode
	networkIntervals     *compactPRISMIntervalIndexNode
	resourceActiveFrom   []float64
	resourceActiveTo     []float64
	pendingPredChunks    [][]uint16
	readyTaskBits        []uint64
	makespan             float64
	cost                 float64
	remainingMinCost     float64
	projectedMakespan    float64
	projectedCost        float64
	queueSeconds         float64
	transferSeconds      float64
	networkCost          float64
	usedResourceCount    int
	signature            uint64
	networkSequence      uint64
}

type compactPRISMTransferResult struct {
	state   compactPRISMState
	readyAt float64
	seconds float64
	cost    float64
}

type compactPRISMFanInDependency struct {
	assignment compactPRISMAssignment
	bytes      int64
}

type compactPRISMFanIn struct {
	dependencies []compactPRISMFanInDependency
}

func initialCompactPRISMState(search compactPRISMContext) compactPRISMState {
	chunkCount := (len(search.activities) + prismPendingChunkSize - 1) / prismPendingChunkSize
	state := compactPRISMState{
		coreAvailable:      compactPRISMInitialCoreRoots(search),
		assignmentChunks:   make([][]compactPRISMAssignment, chunkCount),
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
	return compactPRISMPlacePrepared(
		search,
		state,
		activityOrdinal,
		resourceOrdinal,
		sequence,
		contention,
		compactPRISMPrepareFanIn(search, state, activityOrdinal),
	)
}

func compactPRISMPlacePrepared(
	search compactPRISMContext,
	state compactPRISMState,
	activityOrdinal int,
	resourceOrdinal int,
	sequence int,
	contention bool,
	prepared compactPRISMFanIn,
) compactPRISMState {
	resource := search.resources[resourceOrdinal]
	coreRoot := state.coreAvailable[resourceOrdinal]
	coreOrdinal := coreRoot.bestKey
	transfer := compactPRISMResolvePreparedTransfers(
		search,
		state,
		resource.resource.ID,
		contention,
		prepared,
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
		activityOrdinal,
		resourceOrdinal,
		coreOrdinal,
		sequence,
		order,
		search.activities[activityOrdinal].Priority,
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

func compactPRISMPrepareFanIn(
	search compactPRISMContext,
	state compactPRISMState,
	activityOrdinal int,
) compactPRISMFanIn {
	predecessors := search.predecessors[activityOrdinal]
	prepared := compactPRISMFanIn{
		dependencies: make([]compactPRISMFanInDependency, 0, len(predecessors)),
	}
	for _, dependency := range predecessors {
		assignment, exists := compactPRISMAssignmentChunkLookup(
			state.assignmentChunks,
			dependency.predecessor,
		)
		if !exists {
			continue
		}
		prepared.dependencies = append(prepared.dependencies, compactPRISMFanInDependency{
			assignment: assignment,
			bytes:      dependency.bytes,
		})
	}
	return prepared
}

func compactPRISMResolveTransfers(
	search compactPRISMContext,
	state compactPRISMState,
	activityOrdinal int,
	resourceID string,
	contention bool,
) compactPRISMTransferResult {
	return compactPRISMResolvePreparedTransfers(
		search,
		state,
		resourceID,
		contention,
		compactPRISMPrepareFanIn(search, state, activityOrdinal),
	)
}

func compactPRISMResolvePreparedTransfers(
	search compactPRISMContext,
	state compactPRISMState,
	resourceID string,
	contention bool,
	prepared compactPRISMFanIn,
) compactPRISMTransferResult {
	result := compactPRISMTransferResult{state: state}
	var networkBatch *compactPRISMNetworkBatch
	if contention {
		networkBatch = newCompactPRISMNetworkBatch(state)
	}
	for batchStart := 0; batchStart < len(prepared.dependencies); batchStart += prismPendingChunkSize {
		batchEnd := min(batchStart+prismPendingChunkSize, len(prepared.dependencies))
		for _, dependency := range prepared.dependencies[batchStart:batchEnd] {
			predecessor := dependency.assignment
			flowReadyAt := predecessor.finishAt
			// Large fan-in joins often contain thousands of local or metadata-only
			// dependencies. They only constrain readiness; routing and contention
			// cannot change their zero transfer duration, so resolve them directly.
			predecessorResourceID := search.resources[predecessor.resourceOrdinal].resource.ID
			if predecessorResourceID == resourceID || dependency.bytes <= 0 {
				result.readyAt = math.Max(result.readyAt, flowReadyAt)
				continue
			}
			route, routeExists := search.router.route(predecessorResourceID, resourceID)
			transfer := math.Inf(1)
			if routeExists {
				if contention {
					transfer = compactPRISMTransferSecondsOnRouteBatch(
						networkBatch, predecessorResourceID, resourceID,
						dependency.bytes, flowReadyAt, route,
					)
				} else {
					transfer = compactPRISMTransferSecondsOnRoute(
						compactPRISMState{}, predecessorResourceID, resourceID,
						dependency.bytes, flowReadyAt, route,
					)
				}
			}
			result.seconds += transfer
			if predecessorResourceID != resourceID && dependency.bytes > 0 && routeExists {
				for _, hop := range route.hops {
					result.cost += float64(dependency.bytes) * hop.link.PricePerByte
				}
			}
			deliveredAt := flowReadyAt + transfer
			result.readyAt = math.Max(result.readyAt, deliveredAt)
			if contention && predecessorResourceID != resourceID && dependency.bytes > 0 {
				networkBatch.add(
					predecessorResourceID,
					resourceID,
					flowReadyAt,
					deliveredAt,
					route.hops,
				)
			}
		}
	}
	if contention {
		result.state = networkBatch.commit()
	}
	return result
}

func compactPRISMBuildAssignment(
	activityOrdinal int,
	resourceOrdinal int,
	coreOrdinal int,
	sequence int,
	order int,
	priority int,
	transfer compactPRISMTransferResult,
	start float64,
	runtime float64,
	boot float64,
	container float64,
) compactPRISMAssignment {
	return compactPRISMAssignment{
		activityOrdinal: activityOrdinal, resourceOrdinal: resourceOrdinal,
		coreOrdinal: coreOrdinal, sequence: sequence, order: order, priority: priority,
		readyAt: transfer.readyAt, startAt: start, finishAt: start + runtime,
		runtimeSeconds: runtime, transferSeconds: transfer.seconds,
		bootSeconds: boot, containerSeconds: container,
		queueSeconds: math.Max(0, start-transfer.readyAt-boot-container),
		transferCost: transfer.cost,
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
	assignment compactPRISMAssignment,
) compactPRISMState {
	activity := search.activities[activityOrdinal]
	out.resourceActiveFrom = append([]float64(nil), previous.resourceActiveFrom...)
	out.resourceActiveTo = append([]float64(nil), previous.resourceActiveTo...)
	previousWindow := 0.0
	resourceWasUnused := math.IsInf(out.resourceActiveFrom[resourceOrdinal], 1)
	if !resourceWasUnused {
		previousWindow = out.resourceActiveTo[resourceOrdinal] - out.resourceActiveFrom[resourceOrdinal]
	}
	billingStart := assignment.startAt - assignment.bootSeconds - assignment.containerSeconds
	out.resourceActiveFrom[resourceOrdinal] = math.Min(
		out.resourceActiveFrom[resourceOrdinal],
		billingStart,
	)
	out.resourceActiveTo[resourceOrdinal] = math.Max(
		out.resourceActiveTo[resourceOrdinal],
		assignment.finishAt,
	)
	activeWindow := out.resourceActiveTo[resourceOrdinal] - out.resourceActiveFrom[resourceOrdinal]
	assignment.cost = (activeWindow-previousWindow)*search.resources[resourceOrdinal].resource.PricePerSecond + assignment.transferCost
	out.cost += assignment.cost
	out.assignmentTrace = &compactPRISMAssignmentTrace{
		assignment: assignment,
		previous:   previous.assignmentTrace,
		length:     sequence + 1,
	}
	out.assignmentChunks = compactPRISMAssignmentChunkInsert(
		previous.assignmentChunks,
		activityOrdinal,
		assignment,
	)
	out.coreAvailable = append([]*compactPRISMCoreNode(nil), previous.coreAvailable...)
	out.coreAvailable[resourceOrdinal] = compactPRISMCoreInsert(
		previous.coreAvailable[resourceOrdinal],
		coreOrdinal,
		assignment.finishAt,
	)
	out.coreOrder = compactPRISMIntInsert(previous.coreOrder, coreOrdinal, assignment.order+1)
	out.makespan = math.Max(previous.makespan, assignment.finishAt)
	out.queueSeconds = previous.queueSeconds + assignment.queueSeconds
	out.transferSeconds = previous.transferSeconds + assignment.transferSeconds
	out.networkCost = previous.networkCost + assignment.transferCost
	out.usedResourceCount = previous.usedResourceCount
	if resourceWasUnused {
		out.usedResourceCount++
	}
	out.remainingMinCost -= search.minimumCosts[activityOrdinal]
	out.signature = extendScheduleSignature(
		previous.signature,
		activity.ID,
		search.resources[assignment.resourceOrdinal].resource.ID,
		search.resources[assignment.resourceOrdinal].cores[assignment.coreOrdinal-search.resources[assignment.resourceOrdinal].coreOffset],
	)
	out = compactPRISMAdvanceReady(search, out, activityOrdinal)
	out.projectedCost = out.cost
	// Anchor the remaining upward-rank path at the finish time produced by this
	// actual placement. The previous bound used an unanchored rank, so states
	// with very different core waits frequently tied and were then selected by
	// cost. Subtract this activity's average runtime because its concrete
	// resource-specific runtime is already included in PredictedFinishAt.
	remainingPath := math.Max(
		0,
		search.ranks[activityOrdinal]-search.averageDurations[activityOrdinal],
	)
	out.projectedMakespan = math.Max(out.makespan, assignment.finishAt+remainingPath)
	out.projectedMakespan = math.Max(out.projectedMakespan, compactPRISMReadyRankBound(search, out))
	return out
}

func compactPRISMReadyRankBound(
	search compactPRISMContext,
	state compactPRISMState,
) float64 {
	// Context construction sorts activity ordinals by descending PRISM rank.
	// Therefore the first ready bit is the maximum ready rank.
	for wordIndex, word := range state.readyTaskBits {
		if word == 0 {
			continue
		}
		ordinal := wordIndex*64 + bits.TrailingZeros64(word)
		if ordinal < len(search.ranks) {
			return search.ranks[ordinal]
		}
	}
	return 0
}

func compactPRISMAssignments(search compactPRISMContext, state compactPRISMState) []domain.PlanAssignment {
	if state.evaluatedAssignments != nil {
		return append([]domain.PlanAssignment(nil), state.evaluatedAssignments...)
	}
	if state.assignmentTrace == nil {
		return nil
	}
	items := make([]domain.PlanAssignment, state.assignmentTrace.length)
	for trace := state.assignmentTrace; trace != nil; trace = trace.previous {
		assignment := trace.assignment
		activity := search.activities[assignment.activityOrdinal]
		resource := search.resources[assignment.resourceOrdinal]
		coreID := resource.cores[assignment.coreOrdinal-resource.coreOffset]
		items[trace.length-1] = domain.PlanAssignment{
			ActivityID: activity.ID, ResourceID: resource.resource.ID, CoreID: coreID,
			OrderOnResource: assignment.order, Priority: assignment.priority,
			PredictedReadyAt: assignment.readyAt, PredictedStartAt: assignment.startAt,
			PredictedFinishAt:        assignment.finishAt,
			PredictedRuntimeSeconds:  assignment.runtimeSeconds,
			PredictedTransferSeconds: assignment.transferSeconds,
			PredictedCost:            assignment.cost,
			Metadata: map[string]any{
				"scheduleBasis": "algorithm", "expectedDurationSeconds": assignment.runtimeSeconds,
				"networkContentionModel":   "known-active-flows",
				"bootOverheadSeconds":      assignment.bootSeconds,
				"containerOverheadSeconds": assignment.containerSeconds,
				"queueSeconds":             assignment.queueSeconds, "transferCost": assignment.transferCost,
			},
		}
	}
	return items
}
