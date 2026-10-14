package montagetools

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func stageFixture() (Object, Object) {
	list := func(v ...any) []any { return v }
	rid := "montage-gcp-fog-e2-micro-01"
	file := Object{"path": "data", "sizeBytes": 10, "contentId": "fixture-data"}
	projection := Object{"contract": "whole-workspace-v1", "source": "fixture", "inputs": list(file)}
	req := Object{
		"Run": Object{"mode": "real"},
		"Workflow": Object{
			"activities": list(Object{
				"id":       "new-rootid0000001",
				"name":     "rootid0000001",
				"metadata": Object{"workspaceProjection": projection},
			}, Object{
				"id":       "new-childid0000002",
				"name":     "childid0000002",
				"metadata": Object{"workspaceProjection": Object{"contract": "whole-workspace-v1", "source": "fixture", "outputs": list(Object{"path": "out", "sizeBytes": 2, "contentId": "fixture-out"})}},
			}),
			"dependencies": list(Object{"activityId": "new-childid0000002", "dependsOnActivityId": "new-rootid0000001"}),
		},
		"Resources":       list(Object{"id": rid, "type": "cloud_vm", "schedulable": true, "metadata": Object{}}),
		"Plan":            Object{"assignments": list(Object{"activityId": "new-rootid0000001", "resourceId": rid, "metadata": Object{}}, Object{"activityId": "new-childid0000002", "resourceId": rid, "metadata": Object{}})},
		"NetworkTopology": Object{"metadata": Object{"workspaceMaterializationModel": Object{"mergeProfiles": list(Object{"targetClass": "hpc", "source": "unchanged"})}}},
	}
	training := Object{
		"run": Object{"id": "training-run", "status": "completed"},
		"activities": list(Object{
			"id":                  "root-task",
			"activityId":          "old-rootid0000001",
			"allocatedResourceId": rid,
			"runtimeId":           "cloud",
			"status":              "completed",
			"startedAt":           1,
			"finishedAt":          8,
			"overheadSeconds":     1,
			"metadata":            Object{"launchWaitSeconds": 2},
		}, Object{
			"id":                  "child-task",
			"activityId":          "old-childid0000002",
			"allocatedResourceId": rid,
			"runtimeId":           "cloud",
			"status":              "completed",
			"startedAt":           2,
			"finishedAt":          15,
			"overheadSeconds":     7,
			"metadata":            Object{"launchWaitSeconds": 2, "transferElapsedSeconds": 1000},
		}),
		"handles": list(Object{
			"activityId": "old-rootid0000001",
			"status":     "completed",
			"startedAt":  1,
			"metadata":   Object{},
		}, Object{
			"activityId": "old-childid0000002",
			"status":     "completed",
			"startedAt":  2,
			"metadata":   Object{"remoteWorkspaceSnapshotSeconds": 3},
		}),
		"events": list(Object{
			"eventType":   "activity.completed",
			"aggregateId": "root-task",
			"occurredAt":  "1970-01-01T00:00:10Z",
		}, Object{
			"eventType":   "activity.completed",
			"aggregateId": "child-task",
			"occurredAt":  "1970-01-01T00:00:20Z",
		}),
		"artifactTransferRuns": list(Object{
			"activityId": "old-childid0000002",
			"strategy":   "gateway",
			"status":     "completed",
			"startedAt":  0,
			"route":      Object{"mergeCheckSeconds": 1, "mergeCopySeconds": 2, "mergeVerifySeconds": 3, "mergeCommandOverheadSeconds": 4, "mergeExecutionSeconds": 10},
		}),
	}
	return req, training
}

func TestCloudStageFitDoesNotChargeNetworkResidualOrInitialScanTwice(t *testing.T) {
	r, training := stageFixture()
	out := calibrateRuntimeStages(r, training, "training-hash", "e2-micro")
	model := obj(obj(obj(out["NetworkTopology"])["metadata"])["workspaceMaterializationModel"])
	p := rows(model["inputObservationProfiles"])[0]
	require.Equal(t, 3.0, num(p["fixedSeconds"])+10*num(p["secondsPerUniqueByte"]))
	require.Equal(t, 6.0, num(rows(out["Resources"])[0]["containerOverheadSeconds"])) // 2 launch + (7 overhead - 3 scan)
	require.Equal(t, "simulation", obj(out["Run"])["mode"])
	require.Equal(t, "real", obj(r["Run"])["mode"])
	require.Equal(t, "hpc", rows(model["mergeProfiles"])[0]["targetClass"])
	require.Equal(t, "old-childid0000002", rows(training["activities"])[1]["activityId"])
}

func TestStageFitRejectsMixingRuntimeDrivers(t *testing.T) {
	r, training := stageFixture()
	rows(training["activities"])[0]["runtimeId"] = "other-driver"
	require.Panics(t, func() { calibrateRuntimeStages(r, training, "sha", "e2-micro") })
}
