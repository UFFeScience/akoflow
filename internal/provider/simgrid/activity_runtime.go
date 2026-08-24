package simgrid

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// ActivityRuntime executes a single activity against its simulation model.
// Whole-plan simulation remains responsible for dependencies and transfers;
// this adapter is used by the same lifecycle employed by real and interactive
// execution.
type ActivityRuntime struct{}

func NewActivityRuntime() *ActivityRuntime { return &ActivityRuntime{} }

func (*ActivityRuntime) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeSimulation}
}

func (*ActivityRuntime) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	select {
	case <-ctx.Done():
		return domain.ActivityHandle{}, ctx.Err()
	default:
	}
	if execution.Activity.Simulation == nil {
		return domain.ActivityHandle{}, fmt.Errorf("activity %q has no simulation definition", execution.Activity.ID)
	}
	duration := execution.Activity.Simulation.DurationSeconds
	if duration < 0 {
		return domain.ActivityHandle{}, fmt.Errorf("activity %q simulation duration cannot be negative", execution.Activity.ID)
	}
	return domain.ActivityHandle{
		ID:    fmt.Sprintf("%s:%s:%d", execution.Run.ID, execution.Activity.ID, execution.Run.Seed),
		RunID: execution.Run.ID, ActivityID: execution.Activity.ID,
		ResourceID: execution.Resource.ID, RuntimeID: execution.RuntimeID,
		ExternalID: "simulation", Status: domain.HandleCompleted,
		StartedAt: 0, FinishedAt: duration,
		Metadata: map[string]any{"model": execution.Activity.Simulation.Model},
	}, nil
}

func (*ActivityRuntime) Inspect(_ context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	return handle, nil
}

func (*ActivityRuntime) Stop(context.Context, domain.ActivityHandle) error { return nil }
