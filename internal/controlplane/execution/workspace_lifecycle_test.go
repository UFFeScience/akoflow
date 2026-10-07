package execution

import (
	"net/url"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestBindWorkspaceAllocationPersistsConcreteCloudInstance(t *testing.T) {
	workspace := &domain.ActivityWorkspace{
		URI:       "ssh://cloud-connection/akoflow/workspace/runs/run-1/activity-1",
		RuntimeID: "planned-runtime", ConnectionID: "planned-connection",
	}
	allocation := domain.RuntimeAllocation{
		RuntimeID: "cloud-runtime", ConnectionID: "cloud-connection", CloudInstanceID: "instance-1",
	}
	if err := bindWorkspaceAllocation(workspace, allocation); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(workspace.URI)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.RuntimeID != "cloud-runtime" || workspace.ConnectionID != "cloud-connection" || parsed.Query().Get("cloudInstanceId") != "instance-1" {
		t.Fatalf("workspace was not bound to allocation: %+v", workspace)
	}
}

func TestApplyArtifactManifestClassifiesWorkspaceSnapshots(t *testing.T) {
	workspace := &domain.ActivityWorkspace{}
	manifest := &domain.ArtifactManifest{
		InitialSnapshot: []domain.ArtifactSnapshotEntry{
			{Path: "unchanged.fits", SizeBytes: 10, Checksum: "sha256:same"},
			{Path: "modified.fits", SizeBytes: 20, Checksum: "sha256:old"},
			{Path: "removed.fits", SizeBytes: 30, Checksum: "sha256:removed"},
		},
		FinalSnapshot: []domain.ArtifactSnapshotEntry{
			{Path: "unchanged.fits", SizeBytes: 10, Checksum: "sha256:same"},
			{Path: "modified.fits", SizeBytes: 21, Checksum: "sha256:new"},
			{Path: "created.fits", SizeBytes: 40, Checksum: "sha256:created"},
		},
		Summary: domain.ArtifactSummary{FinalFiles: 3, OutputBytes: 61},
	}
	applyArtifactManifest(workspace, manifest)
	if len(workspace.Manifest.Initial) != 3 || len(workspace.Manifest.Final) != 3 ||
		len(workspace.Manifest.Inputs) != 1 || workspace.Manifest.Inputs[0].Path != "unchanged.fits" ||
		len(workspace.Manifest.Outputs) != 2 || len(workspace.Manifest.Removed) != 1 {
		t.Fatalf("workspace manifest=%+v", workspace.Manifest)
	}
	if workspace.InputBytes != 10 || workspace.OutputBytes != 61 || workspace.FileCount != 3 {
		t.Fatalf("workspace counters=%+v", workspace)
	}
}
