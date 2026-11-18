package transfer

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestInspectStagedWorkspaceVerifiesRecordedContents(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "input.txt")
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(entry, "original")
	expected := []domain.WorkspaceEntry{{Path: "input.txt", Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("original"))), SizeBytes: 8}}
	if err := inspectStagedWorkspace(root, make(map[string][32]byte), expected); err != nil {
		t.Fatal(err)
	}
	write(entry, "modified")
	if err := inspectStagedWorkspace(root, make(map[string][32]byte), expected); err == nil {
		t.Fatal("changed contents passed verification")
	}
	write(entry, "original")
	write(filepath.Join(root, "unexpected.txt"), "unexpected")
	if err := inspectStagedWorkspace(root, make(map[string][32]byte), expected); err == nil {
		t.Fatal("extra file passed verification")
	}
}
