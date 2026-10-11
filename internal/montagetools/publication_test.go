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
	task["startedAt"] = 89.0
	defer func() {
		if recover() == nil {
			t.Fatal("wrong handle attempt accepted")
		}
	}()
	publicationReference(real, "digest")
}
