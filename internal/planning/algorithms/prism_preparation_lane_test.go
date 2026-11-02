package algorithms

import (
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	"testing"
)

func TestPreparationWaitsForLaneBeforeAcquiringService(t *testing.T) {
	flows := []prismEvaluationFlow{{consumer: 0, active: true, stages: []materializationmodel.Stage{{BeforeSeconds: 2, ServiceGroupID: "ssh", ServiceSlots: 1}}}}
	tasks := []prismEvaluationTask{{laneReady: false}}
	pool := map[string]*prismServicePool{"ssh": {slots: 1}}
	if _, err := initializePRISMFlowStages(flows, tasks, pool, compactPRISMRouter{}, 5); err != nil {
		t.Fatal(err)
	}
	if flows[0].stageInitialized || pool["ssh"].used != 0 || flows[0].admitted {
		t.Fatal("blocked lane started preparation or occupied service")
	}
	tasks[0].laneReady = true
	if _, err := initializePRISMFlowStages(flows, tasks, pool, compactPRISMRouter{}, 12); err != nil {
		t.Fatal(err)
	}
	if !flows[0].stageInitialized || flows[0].startedAt != 12 || flows[0].payloadAt != 14 || pool["ssh"].used != 1 {
		t.Fatalf("wrong admitted preparation: %+v", flows[0])
	}
}
