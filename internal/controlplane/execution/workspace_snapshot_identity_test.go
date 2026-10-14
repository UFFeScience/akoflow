package execution

import (
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestSealedWorkspaceSnapshotIdentityRequiresCompleteExactGeneration(t *testing.T) {
	now := time.Now().UTC()
	w := domain.ActivityWorkspace{ID: "workspace", URI: "file:///workspace?cloudInstanceId=vm", State: domain.WorkspaceSealed, SealedAt: &now, FileCount: 2,
		Manifest: domain.WorkspaceManifest{Final: []domain.WorkspaceEntry{
			{Path: "b", SizeBytes: 2, Digest: "sha256:" + strings.Repeat("b", 64)},
			{Path: "a", SizeBytes: 1, Digest: "sha256:" + strings.Repeat("a", 64)},
		}}}
	key, entries := sealedWorkspaceSnapshot(&w, "vm")
	if len(key) != 64 || len(entries) != 2 || entries[0].Path != "a" {
		t.Fatal("missing canonical seal identity")
	}
	w.Manifest.Final[0], w.Manifest.Final[1] = w.Manifest.Final[1], w.Manifest.Final[0]
	if reordered, _ := sealedWorkspaceSnapshot(&w, "vm"); reordered != key {
		t.Fatal("manifest ordering changed identity")
	}
	w.Manifest.Final[0].Digest = "sha256:" + strings.Repeat("c", 64)
	if changed, _ := sealedWorkspaceSnapshot(&w, "vm"); changed == key || changed == "" {
		t.Fatal("changed content failed to invalidate identity")
	}
	if wrong, _ := sealedWorkspaceSnapshot(&w, "other-generation"); wrong != "" {
		t.Fatal("different concrete instance accepted")
	}
	w.State = domain.WorkspaceActive
	if active, _ := sealedWorkspaceSnapshot(&w, "vm"); active != "" {
		t.Fatal("mutable workspace accepted")
	}
	w.State = domain.WorkspaceSealed
	w.FileCount++
	if incomplete, _ := sealedWorkspaceSnapshot(&w, "vm"); incomplete != "" {
		t.Fatal("incomplete manifest accepted")
	}
	w.FileCount--
	w.Manifest.Final[0].Path = "../escape"
	if unsafe, _ := sealedWorkspaceSnapshot(&w, "vm"); unsafe != "" {
		t.Fatal("unsafe manifest path accepted")
	}
}
