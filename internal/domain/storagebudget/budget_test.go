package storagebudget

import (
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/stretchr/testify/require"
)

func fixture(available int64) (workflow.WorkflowVersion, []resource.Resource, map[string]string) {
	file := workflow.ProjectedFile{Path: "data", SizeBytes: 10, ContentID: "source"}
	activity := func(id string, inputs []workflow.ProjectedFile) workflow.Activity {
		return workflow.Activity{ID: id, Metadata: map[string]any{"workspaceProjection": workflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "measured", Inputs: inputs}}}
	}
	v := workflow.WorkflowVersion{Activities: []workflow.Activity{activity("a", []workflow.ProjectedFile{file}), activity("b", nil)}, Dependencies: []workflow.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}}}
	b := Snapshot{StorageID: "shared-home-user-quota", AvailableBytes: available, Source: "quota-api", ObservedAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)}
	r := []resource.Resource{{ID: "node1", Metadata: map[string]any{"workspaceStorageBudget": b}}, {ID: "node2", Metadata: map[string]any{"workspaceStorageBudget": b}}}
	return v, r, map[string]string{"a": "node1", "b": "node2"}
}

func TestSharedQuotaCountsWorkspacesAndStagingNotNominalCapacity(t *testing.T) {
	v, r, a := fixture(40)
	r[0].StorageBytes = 1 << 40
	rows, missing, err := Check(v, r, a)
	require.NoError(t, err)
	require.Empty(t, missing)
	require.Len(t, rows, 1)
	require.Equal(t, int64(40), rows[0].ReservedUpperBoundBytes)
	r[0].Metadata["workspaceStorageBudget"] = r[1].Metadata["workspaceStorageBudget"]
	v, r, a = fixture(0)
	_, _, err = Check(v, r, a)
	require.ErrorContains(t, err, "quota")
}

func TestInconsistentAndMissingQuotaSnapshots(t *testing.T) {
	v, r, a := fixture(40)
	r[1].Metadata["workspaceStorageBudget"] = Snapshot{StorageID: "shared-home-user-quota", AvailableBytes: 80, Source: "quota-api", ObservedAt: time.Now()}
	_, _, err := Check(v, r, a)
	require.ErrorContains(t, err, "inconsistent")
	r[0].Metadata = nil
	r[1].Metadata = nil
	rows, missing, err := Check(v, r, a)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Equal(t, []string{"node1", "node2"}, missing)
}

func TestRootInputRemovedStillReservesPreparationBytes(t *testing.T) {
	v, r, a := fixture(40)
	model := v.Activities[0].Metadata["workspaceProjection"].(workflow.WorkspaceProjection)
	model.RemovedPaths = []string{"data"}
	v.Activities[0].Metadata["workspaceProjection"] = model
	rows, _, err := Check(v, r, a)
	require.NoError(t, err)
	require.Equal(t, int64(10), rows[0].ReservedUpperBoundBytes)
}

func TestScheduledBoundCreditsOnlyReleasedProducersAndRetainsStaging(t *testing.T) {
	v, r, _ := fixture(90)
	model := v.Activities[1].Metadata["workspaceProjection"].(workflow.WorkspaceProjection)
	v.Activities = append(v.Activities, workflow.Activity{ID: "c", Metadata: map[string]any{"workspaceProjection": model}}, workflow.Activity{ID: "d", Metadata: map[string]any{"workspaceProjection": model}})
	v.Dependencies = append(v.Dependencies, workflow.ActivityDependency{ActivityID: "c", DependsOnActivityID: "b"}, workflow.ActivityDependency{ActivityID: "d", DependsOnActivityID: "c"})
	timings := []Timing{{"a", "node1", 0, 1}, {"b", "node2", 2, 3}, {"c", "node1", 4, 5}, {"d", "node2", 6, 7}}
	rows, _, err := CheckScheduled(v, r, timings)
	require.NoError(t, err)
	// 60 staging + retained final 10 + at most two intermediate 10-byte
	// workspaces at equal-time allocation/release barriers = 90.
	require.Equal(t, int64(90), rows[0].ReservedUpperBoundBytes)
	_, _, err = Check(v, r, map[string]string{"a": "node1", "b": "node2", "c": "node1", "d": "node2"})
	require.Error(t, err)
}

func TestScheduledFanoutWaitsForLastConsumerAndRejectsInvalidTiming(t *testing.T) {
	v, r, _ := fixture(1000)
	v.Activities = append(v.Activities, workflow.Activity{ID: "c", Metadata: v.Activities[1].Metadata})
	v.DataDependencies = []workflow.ActivityDataDependency{{ProducerActivityID: "a", ConsumerActivityID: "c"}}
	timings := []Timing{{"a", "node1", 0, 1}, {"b", "node2", 2, 3}, {"c", "node1", 8, 9}}
	rows, _, err := CheckScheduled(v, r, timings)
	require.NoError(t, err)
	require.Equal(t, int64(70), rows[0].ReservedUpperBoundBytes)
	timings[2].Start = .5
	_, _, err = CheckScheduled(v, r, timings)
	require.ErrorContains(t, err, "noncausal")
	_, _, err = CheckScheduled(v, r, timings[:2])
	require.ErrorContains(t, err, "missing storage schedule")
}
