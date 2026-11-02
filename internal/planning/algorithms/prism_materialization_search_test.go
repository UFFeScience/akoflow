package algorithms

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestSearchModelsRelayBeforeRankingAndRoutesNonSchedulableController(t *testing.T) {
	model := materializationmodel.Model{SchemaVersion: 1, Source: "fixture", RelayResourceID: "relay", Resources: map[string]materializationmodel.Resource{
		"source": {Class: "hpc", Domain: "hpc"}, "target": {Class: "micro", Domain: "cloud"}, "bad": {Class: "unmeasured", Domain: "cloud"}, "relay": {Class: "relay", Domain: "relay"},
	}, Profiles: []materializationmodel.Profile{
		{Strategy: "gateway", SourceClass: "hpc", TargetClass: "micro", Source: "fixture", Samples: 1, ArchiveToWorkspaceRatio: 1, IngressIncludesSourcePreparation: true, IngressArchiveBytesPerSecond: 100, UploadArchiveBytesPerSecond: 50},
	}, MergeProfiles: []materializationmodel.MergeProfile{
		{TargetClass: "micro", Source: "fixture", Samples: 1, CheckSecondsPerInputByte: .01},
	}}
	a, b := planningActivity("a", 1), planningActivity("b", 1)
	a.Metadata = map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "fixture", Outputs: []domainworkflow.ProjectedFile{{Path: "file", SizeBytes: 100, ContentID: "file"}}}}
	b.Metadata = map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "fixture"}}
	relay := planningResource("relay", 1, 1, 0)
	relay.Schedulable = false
	request := domain.PlanningRequest{Workflow: domain.WorkflowVersion{
		Activities:   []domain.Activity{a, b},
		Dependencies: []domain.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}},
	}, ExecutionScope: domain.ExecutionScope{
		EnvironmentVersionIDs: []string{"environment"},
	}, Resources: []domain.Resource{
		planningResource("source", 1, 1, 0),
		planningResource("target", 1, 1, 0),
		planningResource("bad", 1, 1, 0),
		relay,
	}, NetworkTopology: domain.NetworkTopology{Metadata: map[string]any{
		"workspaceMaterializationModel": model,
	}, Links: []domain.NetworkLink{
		{ID: "in", SourceResourceID: "source", TargetResourceID: "relay", BandwidthBitsPerSecond: 8e9, SharingPolicy: "SHARED"},
		{ID: "out", SourceResourceID: "relay", TargetResourceID: "target", BandwidthBitsPerSecond: 8e9, SharingPolicy: "SHARED"},
	}}}
	search, err := newCompactPRISMContext(request, nil)
	require.NoError(t, err)
	state := compactPRISMPlace(search, initialCompactPRISMState(search), search.activityOrdinal["a"], compactPRISMResourceOrdinal(search, "source"), 0, true)
	transfer := compactPRISMResolveTransfers(search, state, search.activityOrdinal["b"], "target", true)
	require.InDelta(t, 5, transfer.readyAt, 1e-8)
	state = compactPRISMPlace(search, state, search.activityOrdinal["b"], compactPRISMResourceOrdinal(search, "target"), 1, true)
	require.InDelta(t, 6, state.makespan, 1e-8)
	evaluated, err := evaluateCompleteCompactPRISMState(search, state)
	require.NoError(t, err)
	require.InDelta(t, state.makespan, evaluated.makespan, 1e-8)
	unsupported := compactPRISMResolveTransfers(search, state, search.activityOrdinal["b"], "bad", true)
	require.True(t, math.IsInf(unsupported.readyAt, 1), "unmeasured route must not win search with a guessed direct link")
}
