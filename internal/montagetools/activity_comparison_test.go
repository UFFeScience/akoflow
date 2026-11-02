package montagetools

import "testing"

func comparisonFixture() (Object, Object, Object) {
	id, resource := "montage-mprojectid0000001", "montage-gcp-fog-e2-micro-01"
	request := Object{"Plan": Object{"assignments": []any{Object{"activityId": id, "resourceId": resource, "predictedRuntimeSeconds": 10.0}}}}
	real := Object{
		"run": Object{"id": "reference"},
		"activities": []any{
			Object{
				"id":                  "task-1",
				"activityId":          id,
				"status":              "completed",
				"allocatedResourceId": resource,
				"runtimeSeconds":      10.0,
				"readyAt":             100.0,
				"transferSeconds":     1000.0,
				"queueSeconds":        7.0,
				"metadata":            Object{"transferElapsedSeconds": 3.0},
			},
		},
		"handles": []any{Object{"activityId": id}},
		"events":  []any{Object{"aggregateId": "task-1", "eventType": "activity.completed", "occurredAt": "1970-01-01T00:01:56Z"}},
	}
	sim := Object{"tasks": []any{Object{"activityId": id, "allocatedResourceId": resource, "status": "completed", "readyAt": 0.0, "finishedAt": 14.0, "runtimeSeconds": 10.0}}}
	return request, real, sim
}

func TestActivityComparisonUsesPublishedWallNotParallelWorkSum(t *testing.T) {
	request, real, sim := comparisonFixture()
	report := compareActivityTimes(request, real, sim)
	row := obj(arr(report["activities"])[0])
	if num(row["observedReadyToPublishedSeconds"]) != 16 || num(row["observedTransferWorkSecondsNotElapsed"]) != 1000 || report["computeWithin20PercentCount"] != 1 {
		t.Fatal("wall/compute/work contracts mixed", report)
	}
}

func TestActivityComparisonExcludesRecoveredAndChangedPlacementWall(t *testing.T) {
	request, real, sim := comparisonFixture()
	obj(arr(sim["tasks"])[0])["allocatedResourceId"] = "montage-gcp-fog-e2-micro-02"
	if num(compareActivityTimes(request, real, sim)["samePlacementWallComparableCount"]) != 0 {
		t.Fatal("changed placement presented as pure model residual")
	}
	real["handles"] = append(arr(real["handles"]), Object{"activityId": "montage-mprojectid0000001"})
	if num(compareActivityTimes(request, real, sim)["singleAttemptComparableCount"]) != 0 {
		t.Fatal("recovery presented as independent attempt")
	}
}

func TestActivityComparisonIncludesTransfersBeforeSimgridReadyTimestamp(t *testing.T) {
	request, real, sim := comparisonFixture()
	request["Workflow"] = Object{"dependencies": []any{Object{"activityId": "montage-mprojectid0000001", "dependsOnActivityId": "producer"}}}
	sim["tasks"] = append(arr(sim["tasks"]), Object{"activityId": "producer", "finishedAt": 5.0})
	consumer := obj(arr(sim["tasks"])[0])
	consumer["readyAt"], consumer["finishedAt"] = 10.0, 20.0
	consumer["metadata"] = Object{"workspacePreparationFinishedAt": 12.0}
	row := obj(arr(compareActivityTimes(request, real, sim)["activities"])[0])
	if num(row["simulatedReadyToPublishedSeconds"]) != 15 || num(row["simulatedPreparationElapsedSeconds"]) != 7 {
		t.Fatal("transfer window silently excluded", row)
	}
}
