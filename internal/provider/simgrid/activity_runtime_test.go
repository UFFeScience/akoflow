package simgrid

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestActivityRuntimeProducesCompletedHandle(t *testing.T) {
	runtime := NewActivityRuntime()
	handle, err := runtime.Start(context.Background(), domain.ActivityExecutionContext{
		Run:      domain.ExecutionRun{ID: "run", Seed: 7},
		Activity: domain.Activity{ID: "activity", Simulation: &domain.ActivitySimulation{Model: "profile", DurationSeconds: 12}},
		Resource: domain.Resource{ID: "node"}, RuntimeID: "simgrid",
	})
	if err != nil || handle.Status != domain.HandleCompleted || handle.FinishedAt != 12 || handle.RuntimeID != "simgrid" {
		t.Fatalf("unexpected handle: %+v %v", handle, err)
	}
}
