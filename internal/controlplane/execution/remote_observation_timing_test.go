package execution

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestRemoteObservationEvidenceIsPublishedWithoutDoubleCounting(t *testing.T) {
	task := domain.TaskExecution{OverheadSeconds: 2}
	handle := domain.ActivityHandle{StartedAt: 10, FinishedAt: 20, Metadata: map[string]any{
		"remoteWorkspaceSnapshotSeconds":   7.0,
		"remoteArtifactObservationSeconds": 9.0,
		"remoteArtifactsReadyAt":           29.0,
		"artifactObservationBefore":        "private manifest",
		"artifactObservationRoot":          "/private/path",
	}}
	completeTask(&task, handle)
	if task.RuntimeSeconds != 10 || task.OverheadSeconds != 2 {
		t.Fatal("diagnostics inflated runtime or overhead")
	}
	if task.Metadata["remoteArtifactsReadyAt"] != 29.0 || task.Metadata["remoteWorkspaceSnapshotSeconds"] != 7.0 {
		t.Fatal("timing evidence missing")
	}
	if _, ok := task.Metadata["artifactObservationBefore"]; ok {
		t.Fatal("manifest must not be copied as timing evidence")
	}
	if _, ok := task.Metadata["artifactObservationRoot"]; ok {
		t.Fatal("private path must not be copied as timing evidence")
	}
}
