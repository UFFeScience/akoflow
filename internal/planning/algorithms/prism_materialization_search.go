package algorithms

import (
	"encoding/json"
	"math"

	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

// The search frontier uses conservative projected manifests; complete
// candidates are reevaluated with actual chosen resource-type variants. Both
// use the same causal stages and disallow routes without measured profiles.
func compactPRISMResolveMaterializedFanIn(search compactPRISMContext, state compactPRISMState, target string, contention bool, prepared compactPRISMFanIn) compactPRISMTransferResult {
	result := compactPRISMTransferResult{state: state}
	invalid := func() compactPRISMTransferResult { result.readyAt = math.Inf(1); return result }
	barrier := 0.0
	for _, dependency := range prepared.dependencies {
		barrier = math.Max(barrier, dependency.assignment.finishAt)
	}
	result.readyAt = barrier
	var batch *compactPRISMNetworkBatch
	if contention {
		batch = newCompactPRISMNetworkBatch(state)
	}
	for _, dependency := range prepared.dependencies {
		source := search.resources[dependency.assignment.resourceOrdinal].resource.ID
		materialized, err := search.materializationModel.Route(source, target, dependency.bytes)
		if err != nil {
			return invalid()
		}
		clock := barrier
		for _, stage := range materialized.Stages {
			clock += stage.BeforeSeconds
			if stage.Bytes > 0 {
				route, exists := search.router.route(stage.SourceResourceID, stage.TargetResourceID)
				if !exists {
					return invalid()
				}
				latency := 0.0
				for _, hop := range route.hops {
					latency += hop.link.LatencySeconds
					result.cost += float64(stage.Bytes) * hop.link.PricePerByte
				}
				demand := compactPRISMTransferSecondsOnRoute(compactPRISMState{}, stage.SourceResourceID, stage.TargetResourceID, stage.Bytes, clock, route)
				if contention {
					demand = compactPRISMTransferSecondsOnRouteBatch(batch, stage.SourceResourceID, stage.TargetResourceID, stage.Bytes, clock, route)
				}
				if stage.EffectiveBytesPerSecond > 0 {
					demand = math.Max(demand, latency+float64(stage.Bytes)/stage.EffectiveBytesPerSecond)
				}
				if contention {
					batch.add(stage.SourceResourceID, stage.TargetResourceID, clock, clock+demand, route.hops)
				}
				clock += demand
			}
			clock += stage.AfterSeconds
		}
		result.seconds += clock - barrier
		result.readyAt = math.Max(result.readyAt, clock)
	}
	if len(prepared.dependencies) > 0 {
		var volume domainworkflow.WorkspaceProjectionVolumes
		encoded, err := json.Marshal(search.activities[prepared.activityOrdinal].Metadata["workspaceProjectionVolumes"])
		if err != nil || json.Unmarshal(encoded, &volume) != nil {
			return invalid()
		}
		merge, err := search.materializationModel.PreparationSeconds(target, volume.InputTotalBytes, volume.InputUniqueBytes)
		if err != nil {
			return invalid()
		}
		result.readyAt += merge
	}
	if contention {
		result.state = batch.commit()
	}
	return result
}
