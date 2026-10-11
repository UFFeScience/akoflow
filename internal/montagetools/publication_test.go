package montagetools

import "testing"

func TestPublicationReferenceExcludesPollingAndRejectsWrongAttempt(t *testing.T) {
	handle := Object{"activityId": "a", "status": "completed", "resourceId": "montage-gcp-fog-e2-micro-01", "startedAt": 90.0, "finishedAt": 98.0, "metadata": Object{"executionTarget": "remote-docker", "terminalObservedAt": 100.0}, "artifacts": Object{"finalSnapshot": []any{Object{"sizeBytes": 100.0}}}}
	task := Object{"id": "task-a", "activityId": "a", "status": "completed", "startedAt": 90.0}
	real := Object{"run": Object{"id": "reference", "status": "completed"}, "handles": []any{handle}, "activities": []any{task}, "events": []any{Object{"aggregateId": "task-a", "eventType": "activity.completed", "occurredAt": "1970-01-01T00:01:45Z"}}}
	report := publicationReference(real, "digest")
	row := rows(report["observations"])[0]
	if row["publicationWallSeconds"] != 5.0 || row["pollDelayExcludedSeconds"] != 2.0 {
		t.Fatal("poll wait counted as publication")
	}
	obj(handle["metadata"])["remoteWorkspaceSnapshotSeconds"] = 2.0
	obj(handle["metadata"])["artifactObservationBefore"] = `[{"size":100}]`
	projection := Object{}
	model := Object{}
	request := Object{"Workflow": Object{"activities": []any{Object{"id": "a", "metadata": Object{"workspaceProjection": projection}}}}, "NetworkTopology": Object{"metadata": Object{"workspaceMaterializationModel": model}}}
	combinedObservationSensitivity(request, real, "digest")
	if projection["terminalObservationBytes"] != int64(100) || rows(model["publicationProfiles"])[0]["scanSecondsPerWorkspaceByte"] != .05 {
		t.Fatal("publication scan used exported size or wrong wall coefficient")
	}
	task["startedAt"] = 89.0
	defer func() {
		if recover() == nil {
			t.Fatal("wrong handle attempt accepted")
		}
	}()
	publicationReference(real, "digest")
}
