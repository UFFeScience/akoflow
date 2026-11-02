package transfer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingleProcessMergePreservesWholeWorkspaceAndRejectsConflicts(t *testing.T) {
	for _, mode := range []string{"mixed", "conflict", "symlink", "directory-conflict"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			a, b, dest := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "destination")
			for _, path := range []string{a, b, dest} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			name := "space ' quote\nnewline.fits"
			for _, path := range []string{a, b} {
				if err := os.WriteFile(filepath.Join(path, name), []byte("same"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(a, "only-a"), []byte("a"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(b, "only-b"), []byte("b"), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "conflict" {
				if err := os.WriteFile(filepath.Join(b, name), []byte("different"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "symlink" {
				if err := os.Symlink(filepath.Join(a, name), filepath.Join(b, "link")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "directory-conflict" {
				if err := os.MkdirAll(filepath.Join(b, "only-a"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			output, err := exec.Command("bash", "-c", remoteWorkspaceScript([]string{a, b}, dest)).CombinedOutput()
			if mode != "mixed" {
				if err == nil {
					t.Fatalf("unsafe source accepted: %s", output)
				}
				entries, readErr := os.ReadDir(dest)
				if readErr != nil || len(entries) != 0 {
					t.Fatal("failure published files before validation")
				}
				return
			}
			if err != nil {
				t.Fatalf("merge: %v %s", err, output)
			}
			if !strings.Contains(string(output), "__AKOFLOW_MERGE_VERIFIED_FILES__=3") {
				t.Fatalf("whole workspace not verified: %s", output)
			}
			// Recovery must safely reuse the already verified whole workspace.
			if output, err := exec.Command("bash", "-c", remoteWorkspaceScript([]string{a, b}, dest)).CombinedOutput(); err != nil {
				t.Fatalf("idempotent recovery: %v %s", err, output)
			}
			if err := os.WriteFile(filepath.Join(dest, name), []byte("consumer changed"), 0600); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(a, name))
			if err != nil || string(content) != "same" {
				t.Fatal("consumer copy modified producer")
			}
		})
	}
}

func TestRemoteMergeSecondsRejectsInvalidTelemetry(t *testing.T) {
	for _, value := range []string{"-1", "NaN", "+Inf", "garbage"} {
		if got := remoteMergeSeconds("__AKOFLOW_MERGE_CHECK_SECONDS__="+value, "CHECK"); got != 0 {
			t.Fatalf("accepted %s: %v", value, got)
		}
	}
	if got := remoteMergeSeconds("__AKOFLOW_MERGE_CHECK_SECONDS__=1.25\n", "CHECK"); got != 1.25 {
		t.Fatalf("got %v", got)
	}
}
