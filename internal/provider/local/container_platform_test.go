package local

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestContainerRetriesVerifiedForeignPlatform(t *testing.T) {
	if runtime.GOOS == "windows" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		t.Skip("requires a POSIX host with a supported platform")
	}
	target := "arm64"
	if runtime.GOARCH == target {
		target = "amd64"
	}
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$3" = linux/` + runtime.GOARCH + ` ]; then
  echo 'no matching manifest for the host platform' >&2
  exit 125
elif [ "$3" = linux/` + target + ` ]; then
  echo proof-container
else
  exit 1
fi
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	output, warning, err := startContainerWithRegistry(context.Background(), []string{"run", "example/image:1"}, "example/image:1",
		func(context.Context, string) []string { return []string{"linux/" + target} })
	if err != nil || strings.TrimSpace(string(output)) != "proof-container" || !strings.Contains(warning, "linux/"+target) || !strings.Contains(warning, "emulation") {
		t.Fatalf("unexpected fallback: %q %q %v", output, warning, err)
	}
}
