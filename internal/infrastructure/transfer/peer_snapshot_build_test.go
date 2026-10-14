package transfer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestPeerSnapshotManifestRejectsMutableOrIncompleteContents(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"valid", "changed", "missing", "extra", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			data := []byte("original")
			entry := domain.WorkspaceEntry{Path: "file", SizeBytes: int64(len(data)), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(data))}
			if kind == "changed" {
				data = []byte("modified")
			}
			if kind != "missing" {
				if err := os.WriteFile(filepath.Join(root, "file"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "extra" {
				if err := os.WriteFile(filepath.Join(root, "extra"), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "symlink" {
				if err := os.Symlink("file", filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
			}
			manifest, _ := json.Marshal([]domain.WorkspaceEntry{entry})
			output, err := exec.Command(python, "-c", peerSnapshotManifestScript, root, base64.StdEncoding.EncodeToString(manifest)).CombinedOutput()
			if (kind == "valid") != (err == nil) {
				t.Fatalf("validation %s: %v %s", kind, err, output)
			}
		})
	}
}

func TestPeerSnapshotKeyInvalidatesGenerationPathAndManifest(t *testing.T) {
	source := domain.TransferEndpoint{URI: "ssh://worker/workspace?hostKeyAlias=generation", ConnectionID: "ssh-vm", CloudInstanceID: "vm", WorkspaceSnapshotID: strings.Repeat("a", 64), WorkspaceSnapshotEntries: []domain.WorkspaceEntry{
		{Path: "file", Digest: "sha256:" + strings.Repeat("b", 64), SizeBytes: 1},
	}}
	key := peerSnapshotKey(source)
	if key == "" {
		t.Fatal("complete snapshot has no identity")
	}
	for _, change := range []func(*domain.TransferEndpoint){
		func(s *domain.TransferEndpoint) { s.CloudInstanceID = "other" },
		func(s *domain.TransferEndpoint) { s.URI += "-other" },
		func(s *domain.TransferEndpoint) { s.WorkspaceSnapshotID = strings.Repeat("c", 64) },
		func(s *domain.TransferEndpoint) {
			s.WorkspaceSnapshotEntries = []domain.WorkspaceEntry{{Path: "changed"}}
		},
	} {
		copy := source
		change(&copy)
		if peerSnapshotKey(copy) == key {
			t.Fatal("changed identity reused")
		}
	}
	source.WorkspaceSnapshotID = ""
	if peerSnapshotKey(source) != "" {
		t.Fatal("unknown seal cached")
	}
}
