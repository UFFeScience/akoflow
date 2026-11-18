package workflow_engine_api_handler

import (
	"reflect"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDescribeActivityTestMatchesInputsFromTheOriginalRun(t *testing.T) {
	workflow := domain.WorkflowVersion{
		Activities: []domain.Activity{{ID: "producer"}, {ID: "other"}, {ID: "target"}, {ID: "downstream"}},
		Dependencies: []domain.ActivityDependency{
			{ActivityID: "target", DependsOnActivityID: "producer"},
			{ActivityID: "target", DependsOnActivityID: "other"},
			{ActivityID: "downstream", DependsOnActivityID: "target"},
		},
	}
	manifest := &domain.ArtifactManifest{InitialSnapshot: []domain.ArtifactSnapshotEntry{
		{Path: "original.txt", Checksum: "sha256:original", SizeBytes: 3},
		{Path: "changed.txt", Checksum: "sha256:before", SizeBytes: 8},
	}}
	sealed := time.Now()
	workspaces := []domain.ActivityWorkspace{
		{RunID: "source", ActivityID: "producer", State: domain.WorkspaceSealed, SealedAt: &sealed, FileCount: 1,
			Manifest: domain.WorkspaceManifest{Final: []domain.WorkspaceEntry{{Path: "original.txt", Digest: "sha256:original", SizeBytes: 3}}}},
		{RunID: "source", ActivityID: "other", State: domain.WorkspaceSealed, SealedAt: &sealed, FileCount: 1,
			Manifest: domain.WorkspaceManifest{Final: []domain.WorkspaceEntry{{Path: "changed.txt", Digest: "sha256:after", SizeBytes: 8}}}},
		{RunID: "foreign", ActivityID: "other", State: domain.WorkspaceSealed, SealedAt: &sealed, FileCount: 1,
			Manifest: domain.WorkspaceManifest{Final: []domain.WorkspaceEntry{{Path: "changed.txt", Digest: "sha256:before", SizeBytes: 8}}}},
	}

	preview, err := describeActivityTest(workflow, "source", "target", 2, manifest, workspaces)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Attempt != 2 || !reflect.DeepEqual(preview.RunActivityIDs, []string{"other", "target"}) ||
		!reflect.DeepEqual(preview.ReuseActivityIDs, []string{"producer"}) ||
		!reflect.DeepEqual(preview.Missing, []string{"changed.txt"}) ||
		len(preview.Inputs) != 2 || preview.Inputs[0].Producer != "producer" || preview.Inputs[1].Producer != "" {
		t.Fatalf("unexpected activity test preview: %+v", preview)
	}
}
