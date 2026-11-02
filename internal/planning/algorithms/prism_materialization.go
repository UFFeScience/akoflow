package algorithms

import "fmt"

// Each relay leg is a separate network demand. Preparation and receiver work
// are causal delays around that leg, not an extra end-to-end payload.
func prismInitializeMaterializationStage(flow *prismEvaluationFlow, router compactPRISMRouter, clock float64) (float64, error) {
	stage := flow.stages[flow.stageIndex]
	flow.source, flow.target = stage.SourceResourceID, stage.TargetResourceID
	flow.bytes, flow.remaining = stage.Bytes, float64(stage.Bytes)
	flow.rateLimit = stage.EffectiveBytesPerSecond
	flow.postStage = false
	flow.payloadAt = clock + stage.BeforeSeconds
	flow.route = compactPRISMRoute{}
	if stage.Bytes == 0 {
		return 0, nil
	}
	route, exists := router.route(flow.source, flow.target)
	if !exists {
		return 0, fmt.Errorf("materialization stage %s has no route from %s to %s", stage.Name, flow.source, flow.target)
	}
	flow.route = route
	cost := 0.0
	for _, hop := range route.hops {
		flow.payloadAt += hop.link.LatencySeconds
		cost += float64(stage.Bytes) * hop.link.PricePerByte
	}
	return cost, nil
}
