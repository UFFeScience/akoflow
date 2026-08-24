package simgrid

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSimulationExecutesFrozenPlanWithRealNetworkModel(t *testing.T) {
	workflow := domain.WorkflowVersion{
		ID: "wf-v1",
		Activities: []domain.Activity{
			{ID: "a", ActivityTypeID: "type-a", Resources: domain.ActivityResources{CPU: 1, MemoryBytes: 1}},
			{ID: "b", ActivityTypeID: "type-b", Resources: domain.ActivityResources{CPU: 1, MemoryBytes: 1}},
		},
		Dependencies:     []domain.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}},
		DataDependencies: []domain.ActivityDataDependency{{ProducerActivityID: "a", ConsumerActivityID: "b", SizeBytes: 1_000_000}},
	}
	resources := []domain.Resource{
		{ID: "edge", EnvironmentVersionID: "env-v1", CPUCapacity: 1, MemoryBytes: 1, ComputeSpeedup: 1, Schedulable: true},
		{ID: "cloud", EnvironmentVersionID: "env-v1", CPUCapacity: 1, MemoryBytes: 1, ComputeSpeedup: 1, Schedulable: true},
	}
	plan := domain.SchedulePlan{
		ID: "plan", WorkflowVersionID: "wf-v1", ExecutionScopeID: "scope",
		DeadlineSeconds: 10,
		Assignments: []domain.PlanAssignment{
			{ID: "pa", ActivityID: "a", ResourceID: "edge", OrderOnResource: 0},
			{ID: "pb", ActivityID: "b", ResourceID: "cloud", OrderOnResource: 0},
		},
	}
	profiles := []domain.ActivityResourceProfile{
		{ActivityTypeID: "type-a", ResourceID: "edge", RuntimeSeconds: 2},
		{ActivityTypeID: "type-b", ResourceID: "cloud", RuntimeSeconds: 3},
	}
	trace, err := NewSimulationExecutor().Execute(context.Background(), Request{
		Run:  domain.ExecutionRun{ID: "run", Mode: domain.ExecutionModeSimulation},
		Plan: plan, Workflow: workflow, Resources: resources, ActivityProfiles: profiles,
		ExecutionScope: domain.ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"env-v1"}},
		NetworkTopology: domain.NetworkTopology{ID: "topology", ExecutionScopeID: "scope", Links: []domain.NetworkLink{{
			TopologyID: "topology", SourceResourceID: "edge", TargetResourceID: "cloud",
			Bidirectional: true, BandwidthBitsPerSecond: 8_000_000, LatencySeconds: 0.1,
		}}},
	})
	require.NoError(t, err)
	require.Len(t, trace.Transfers, 1)
	require.InDelta(t, 1.1, trace.Transfers[0].DurationSeconds, 1e-9)
	require.InDelta(t, 6.1, trace.Executed.MakespanSeconds, 1e-9)
	require.True(t, trace.Executed.Feasible)
}

func TestSimulationResolvesMultiHopNetworkPath(t *testing.T) {
	duration, cost, ok := resolveNetworkPath([]domain.NetworkLink{
		{SourceResourceID: "edge", TargetResourceID: "gateway", Bidirectional: true, BandwidthBitsPerSecond: 8_000_000, LatencySeconds: .1, PricePerByte: 1e-9},
		{SourceResourceID: "gateway", TargetResourceID: "cloud", Bidirectional: true, BandwidthBitsPerSecond: 16_000_000, LatencySeconds: .2, PricePerByte: 2e-9},
	}, "edge", "cloud", 1_000_000)
	require.True(t, ok)
	require.InDelta(t, 1.8, duration, 1e-9)
	require.InDelta(t, .003, cost, 1e-9)
}

func TestSimulationUsesAssignmentOverheadAndBillsResourceActiveWindowOnce(t *testing.T) {
	workflow := domain.WorkflowVersion{ID: "wf", Activities: []domain.Activity{
		{ID: "a", ActivityTypeID: "type", Metadata: map[string]any{"baseRuntimeSeconds": 2}},
		{ID: "b", ActivityTypeID: "type", Metadata: map[string]any{"baseRuntimeSeconds": 2}},
	}}
	resource := domain.Resource{ID: "vm", EnvironmentVersionID: "env",
		CPUCapacity: 2, MemoryBytes: 1, ComputeSpeedup: 1, PricePerSecond: 1,
		BootOverheadSeconds: 100, ContainerOverhead: 100, Schedulable: true}
	plan := domain.SchedulePlan{ID: "plan", WorkflowVersionID: "wf", ExecutionScopeID: "scope",
		Assignments: []domain.PlanAssignment{
			{ID: "a", ActivityID: "a", ResourceID: "vm", CoreID: "0", Metadata: map[string]any{"bootOverheadSeconds": 3, "containerOverheadSeconds": 1}},
			{ID: "b", ActivityID: "b", ResourceID: "vm", CoreID: "1", Metadata: map[string]any{"bootOverheadSeconds": 0, "containerOverheadSeconds": 1}},
		}}
	trace, err := NewSimulationExecutor().Execute(context.Background(), Request{
		Run: domain.ExecutionRun{ID: "run", Mode: domain.ExecutionModeSimulation}, Plan: plan,
		Workflow: workflow, Resources: []domain.Resource{resource},
		ExecutionScope:  domain.ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"env"}},
		NetworkTopology: domain.NetworkTopology{ExecutionScopeID: "scope"},
	})
	require.NoError(t, err)
	require.InDelta(t, 6, trace.Executed.MakespanSeconds, 1e-9)
	require.InDelta(t, 6, trace.Executed.Cost, 1e-9)
}

func TestSimulationRejectsPlanThatDoesNotCoverWorkflow(t *testing.T) {
	workflow := domain.WorkflowVersion{ID: "wf", Activities: []domain.Activity{{ID: "a"}}}
	_, err := NewSimulationExecutor().Execute(context.Background(), Request{
		Run:  domain.ExecutionRun{Mode: domain.ExecutionModeSimulation},
		Plan: domain.SchedulePlan{WorkflowVersionID: "wf", ExecutionScopeID: "scope"}, Workflow: workflow,
		ExecutionScope:  domain.ExecutionScope{ID: "scope"},
		NetworkTopology: domain.NetworkTopology{ExecutionScopeID: "scope"},
	})
	require.ErrorContains(t, err, "covers 0 of 1 activities")
}
