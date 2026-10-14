package workflow

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestWorkspaceProjectionReconstructsAuditedMontageBaseline(t *testing.T) {
	payload, err := os.ReadFile("../../../experiments/montage-micro-hpc-e2/workspace-projection-v2.json")
	require.NoError(t, err)
	var evidence struct {
		ActivityModels map[string]WorkspaceProjection `json:"activityModels"`
		Dependencies   []ActivityDependency           `json:"dependencies"`
		ObservedBytes  map[string]int64               `json:"observedProducerWorkspaceBytes"`
		ResourceTypes  map[string]string              `json:"baselineAssignedResourceTypes"`
	}
	require.NoError(t, json.Unmarshal(payload, &evidence))
	require.Len(t, evidence.ActivityModels, 58)
	version := WorkflowVersion{Dependencies: evidence.Dependencies}
	for id, model := range evidence.ActivityModels {
		version.Activities = append(version.Activities, Activity{ID: id, Metadata: map[string]any{"workspaceProjection": model}})
	}
	projected, err := ProjectWorkspaceDependenciesForResourceTypes(version, evidence.ResourceTypes)
	require.NoError(t, err)
	require.Len(t, projected.DataDependencies, 114)
	for _, edge := range projected.DataDependencies {
		require.Equal(t, evidence.ObservedBytes[edge.ProducerActivityID], edge.SizeBytes, "producer %s consumer %s", edge.ProducerActivityID, edge.ConsumerActivityID)
	}
	// Change a root's future placement: inherited image packaging must change
	// the downstream predictions, not replay the previous assignment's bytes.
	for id, model := range evidence.ActivityModels {
		if len(model.InputsByResourceType) == 0 || evidence.ResourceTypes[id] != "hpc_machine" {
			continue
		}
		evidence.ResourceTypes[id] = "cloud_vm"
		swapped, err := ProjectWorkspaceDependenciesForResourceTypes(version, evidence.ResourceTypes)
		require.NoError(t, err)
		for _, edge := range swapped.DataDependencies {
			if edge.ProducerActivityID == id {
				require.Greater(t, edge.SizeBytes, evidence.ObservedBytes[id])
			}
		}
		break
	}
}

func projectedActivity(id string, inputs, outputs []ProjectedFile) Activity {
	return Activity{ID: id, Metadata: map[string]any{"workspaceProjection": WorkspaceProjection{Contract: "whole-workspace-v1", Source: "completed-baseline", Inputs: inputs, Outputs: outputs}}}
}

func TestWorkspaceProjectionInheritedDiamond(t *testing.T) {
	input := ProjectedFile{Path: "input.fits", SizeBytes: 100, ContentID: "input-hash"}
	version := WorkflowVersion{Activities: []Activity{
		projectedActivity("a", []ProjectedFile{input}, nil),
		projectedActivity("b", nil, []ProjectedFile{{Path: "b.fits", SizeBytes: 20, ContentID: "expected-b"}}),
		projectedActivity("c", nil, []ProjectedFile{{Path: "c.fits", SizeBytes: 30, ContentID: "expected-c"}}),
		projectedActivity("d", nil, nil), projectedActivity("e", nil, nil),
	}, Dependencies: []ActivityDependency{
		{ActivityID: "b", DependsOnActivityID: "a"},
		{ActivityID: "c", DependsOnActivityID: "a"},
		{ActivityID: "d", DependsOnActivityID: "b"},
		{ActivityID: "d", DependsOnActivityID: "c"},
		{ActivityID: "e", DependsOnActivityID: "d"},
	}}
	projected, err := ProjectWorkspaceDependencies(version)
	require.NoError(t, err)
	var sizes []int64
	for _, edge := range projected.DataDependencies {
		sizes = append(sizes, edge.SizeBytes)
	}
	// Both parent snapshots include input; the merged child has it only once.
	require.Equal(t, []int64{100, 100, 120, 130, 150}, sizes)
	for _, activity := range projected.Activities {
		if activity.ID == "d" {
			volume := activity.Metadata["workspaceProjectionVolumes"].(WorkspaceProjectionVolumes)
			require.EqualValues(t, 250, volume.InputTotalBytes)
			require.EqualValues(t, 150, volume.InputUniqueBytes)
			require.Equal(t, 3, volume.InputFiles)
		}
	}
	for _, activity := range version.Activities {
		require.NotContains(t, activity.Metadata, "workspaceProjectionVolumes")
	}
	require.Empty(t, version.DataDependencies, "must not mutate the catalog snapshot")
	second, err := ProjectWorkspaceDependencies(projected)
	require.NoError(t, err)
	require.Equal(t, projected.DataDependencies, second.DataDependencies)
}

func TestWorkspaceProjectionRejectsConflictingParents(t *testing.T) {
	version := WorkflowVersion{
		Activities: []Activity{
			projectedActivity("a", nil, []ProjectedFile{{Path: "same", SizeBytes: 1, ContentID: "a"}}),
			projectedActivity("b", nil, []ProjectedFile{{Path: "same", SizeBytes: 1, ContentID: "b"}}),
			projectedActivity("c", nil, nil),
		},
		Dependencies: []ActivityDependency{{ActivityID: "c", DependsOnActivityID: "a"}, {ActivityID: "c", DependsOnActivityID: "b"}},
	}
	_, err := ProjectWorkspaceDependencies(version)
	require.ErrorContains(t, err, "conflict")
}

func TestWorkspaceProjectionFailsClosed(t *testing.T) {
	for _, file := range []ProjectedFile{{Path: "../secret", SizeBytes: 1, ContentID: "x"}, {Path: "/absolute", SizeBytes: 1, ContentID: "x"}, {Path: "valid", SizeBytes: -1, ContentID: "x"}, {Path: "valid", SizeBytes: 1}} {
		_, err := ProjectWorkspaceDependencies(WorkflowVersion{Activities: []Activity{projectedActivity("a", []ProjectedFile{file}, nil)}})
		require.Error(t, err)
	}
	_, err := ProjectWorkspaceDependencies(WorkflowVersion{Activities: []Activity{projectedActivity("a", nil, nil), {ID: "b"}}})
	require.ErrorContains(t, err, "missing")
	_, err = ProjectWorkspaceDependencies(WorkflowVersion{Activities: []Activity{projectedActivity("a", nil, nil)}, Dependencies: []ActivityDependency{{ActivityID: "a", DependsOnActivityID: "a"}}})
	require.ErrorContains(t, err, "cycle")
}

func TestWorkspaceProjectionLegacyUnchanged(t *testing.T) {
	version := WorkflowVersion{Activities: []Activity{{ID: "a"}}, DataDependencies: []ActivityDataDependency{{SizeBytes: 42}}}
	actual, err := ProjectWorkspaceDependencies(version)
	require.NoError(t, err)
	require.Equal(t, version, actual)
}

func TestWorkspaceProjectionRuntimePackagingUsesAssignedType(t *testing.T) {
	output := ProjectedFile{Path: "projected.fits", SizeBytes: 100, ContentID: "science-output"}
	asset := ProjectedFile{Path: "image-input.fits", SizeBytes: 90, ContentID: "image-input"}
	a := projectedActivity("a", []ProjectedFile{asset}, []ProjectedFile{output})
	model := a.Metadata["workspaceProjection"].(WorkspaceProjection)
	model.InputsByResourceType = map[string][]ProjectedFile{"cloud_vm": {asset}, "hpc_machine": {}}
	a.Metadata["workspaceProjection"] = model
	version := WorkflowVersion{Activities: []Activity{a, projectedActivity("b", nil, nil)}, Dependencies: []ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}}}
	search, err := ProjectWorkspaceDependencies(version)
	require.NoError(t, err)
	require.EqualValues(t, 190, search.DataDependencies[0].SizeBytes)
	hpc, err := ProjectWorkspaceDependenciesForResourceTypes(version, map[string]string{"a": "hpc_machine", "b": "cloud_vm"})
	require.NoError(t, err)
	require.EqualValues(t, 100, hpc.DataDependencies[0].SizeBytes)
	_, err = ProjectWorkspaceDependenciesForResourceTypes(version, map[string]string{"a": "unknown"})
	require.ErrorContains(t, err, "runtime inputs")
}
