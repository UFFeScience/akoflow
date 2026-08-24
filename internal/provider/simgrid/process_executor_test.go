package simgrid

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
)

type runnerCommandFake struct {
	name  string
	args  []string
	trace domain.ExecutionTrace
}

func (f *runnerCommandFake) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, error) {
	f.name, f.args = name, append([]string(nil), args...)
	output := argumentValue(args, "--output")
	payload, err := json.Marshal(f.trace)
	if err != nil {
		return nil, err
	}
	return []byte("runner completed"), os.WriteFile(output, payload, 0o600)
}

func TestProcessExecutorBuildsBundleWaitsAndReadsTrace(t *testing.T) {
	request := processRequestFixture()
	command := &runnerCommandFake{trace: domain.ExecutionTrace{
		RunID: request.Run.ID, PlanID: request.Plan.ID,
		Executed: domain.ExecutionMetrics{MakespanSeconds: 12, Feasible: true},
	}}
	executor, err := NewProcessExecutor(command, ProcessConfig{
		BinaryPath: "/usr/local/bin/akoflow-simgrid-runner",
		Workspace:  t.TempDir(), MaxConcurrent: 2, Timeout: time.Minute,
	})
	require.NoError(t, err)
	trace, err := executor.Execute(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, 12.0, trace.Executed.MakespanSeconds)
	require.Equal(t, domain.ExecutionModeSimulation, trace.Mode)
	require.Equal(t, command.name, "/usr/local/bin/akoflow-simgrid-runner")

	platform := argumentValue(command.args, "--platform")
	input := argumentValue(command.args, "--input")
	require.FileExists(t, platform)
	require.FileExists(t, input)
	require.FileExists(t, filepath.Join(filepath.Dir(input), "runner.log"))
	platformPayload, err := os.ReadFile(platform)
	require.NoError(t, err)
	require.Contains(t, string(platformPayload), `bandwidth="62500000.000000000Bps"`)
	require.Contains(t, string(platformPayload), `<route src="cloud" dst="edge" symmetrical="NO">`)
}

func TestPlatformBuildsMultiHopRoutes(t *testing.T) {
	resources := []domain.Resource{
		{ID: "edge", ComputeSpeedup: 1}, {ID: "gateway", ComputeSpeedup: 2}, {ID: "cloud", ComputeSpeedup: 4},
	}
	topology := domain.NetworkTopology{Links: []domain.NetworkLink{
		{SourceResourceID: "edge", TargetResourceID: "gateway", BandwidthBitsPerSecond: 8e6, Bidirectional: true},
		{SourceResourceID: "gateway", TargetResourceID: "cloud", BandwidthBitsPerSecond: 16e6, Bidirectional: true},
	}}
	payload, err := buildPlatformXML(resources, topology, 1e9)
	require.NoError(t, err)
	require.Contains(t, string(payload), `<route src="edge" dst="cloud" symmetrical="NO">`)
	require.Contains(t, string(payload), `<link_ctn id="link-0"/>`)
	require.Contains(t, string(payload), `<link_ctn id="link-1"/>`)
}

func processRequestFixture() Request {
	return Request{
		Run: domain.ExecutionRun{ID: "run", Mode: domain.ExecutionModeSimulation},
		Plan: domain.SchedulePlan{
			ID: "plan", WorkflowVersionID: "workflow", ExecutionScopeID: "scope",
			Assignments: []domain.PlanAssignment{
				{ID: "assignment-a", ActivityID: "a", ResourceID: "edge", OrderOnResource: 1},
				{ID: "assignment-b", ActivityID: "b", ResourceID: "cloud", OrderOnResource: 1},
			},
		},
		Workflow: domain.WorkflowVersion{
			ID: "workflow",
			Activities: []domain.Activity{
				{ID: "a", ActivityTypeID: "type", Simulation: &domain.ActivitySimulation{DurationSeconds: 2}},
				{ID: "b", ActivityTypeID: "type", Simulation: &domain.ActivitySimulation{DurationSeconds: 3}},
			},
			Dependencies: []domain.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}},
			DataDependencies: []domain.ActivityDataDependency{{
				ProducerActivityID: "a", ConsumerActivityID: "b", SizeBytes: 1_000_000,
			}},
		},
		Resources: []domain.Resource{
			{ID: "edge", EnvironmentVersionID: "environment", ComputeSpeedup: 1},
			{ID: "cloud", EnvironmentVersionID: "environment", ComputeSpeedup: 4},
		},
		NetworkTopology: domain.NetworkTopology{Links: []domain.NetworkLink{{
			SourceResourceID: "edge", TargetResourceID: "cloud",
			BandwidthBitsPerSecond: 500e6, LatencySeconds: .1, Bidirectional: true,
		}}},
	}
}

func argumentValue(arguments []string, name string) string {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == name {
			return arguments[index+1]
		}
	}
	return ""
}
