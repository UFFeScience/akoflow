package workflow

import (
	"github.com/stretchr/testify/require"
	"testing"
)

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
	}, Dependencies: []ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}, {ActivityID: "c", DependsOnActivityID: "a"}, {ActivityID: "d", DependsOnActivityID: "b"}, {ActivityID: "d", DependsOnActivityID: "c"}, {ActivityID: "e", DependsOnActivityID: "d"}}}
	projected, err := ProjectWorkspaceDependencies(version)
	require.NoError(t, err)
	var sizes []int64
	for _, edge := range projected.DataDependencies {
		sizes = append(sizes, edge.SizeBytes)
	}
	// Both parent snapshots include input; the merged child has it only once.
	require.Equal(t, []int64{100, 100, 120, 130, 150}, sizes)
	require.Empty(t, version.DataDependencies, "must not mutate the catalog snapshot")
	second, err := ProjectWorkspaceDependencies(projected)
	require.NoError(t, err)
	require.Equal(t, projected.DataDependencies, second.DataDependencies)
}

func TestWorkspaceProjectionRejectsConflictingParents(t *testing.T) {
	version := WorkflowVersion{Activities: []Activity{projectedActivity("a", nil, []ProjectedFile{{Path: "same", SizeBytes: 1, ContentID: "a"}}), projectedActivity("b", nil, []ProjectedFile{{Path: "same", SizeBytes: 1, ContentID: "b"}}), projectedActivity("c", nil, nil)}, Dependencies: []ActivityDependency{{ActivityID: "c", DependsOnActivityID: "a"}, {ActivityID: "c", DependsOnActivityID: "b"}}}
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
