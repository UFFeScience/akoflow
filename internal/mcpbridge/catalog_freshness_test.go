package mcpbridge

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The existing Go CI job runs this check, without requiring a separate workflow.
// Generate in a temporary source tree so tests never rewrite a user's checkout.
func TestGeneratedCatalogMatchesDaemonRoutesAndRequestTypes(t *testing.T) {
	root := t.TempDir()
	source, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	if err = copyCatalogSources(source, filepath.Join(root, "internal")); err != nil {
		t.Fatal(err)
	}
	module := "module github.com/UFFeScience/akoflow\n\ngo 1.25.0\n"
	if err = os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	command := exec.Command(executable, "run", "./cataloggen")
	command.Dir = filepath.Join(root, "internal", "mcpbridge")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate catalog: %v\n%s", err, output)
	}
	generated, err := os.ReadFile(filepath.Join(command.Dir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, catalogJSON) {
		t.Fatal("MCP catalog is stale; run go generate ./internal/mcpbridge and commit catalog.json")
	}
}
func copyCatalogSources(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0600)
	})
}
