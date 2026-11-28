package local

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestContainerImageObservationUsesTheStartedContainerAcrossTagChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a shell for the Docker fixture")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	first := "sha256:" + strings.Repeat("a", 64)
	second := "sha256:" + strings.Repeat("b", 64)
	script := `#!/bin/sh
case "$1:$2:$5" in
 container:inspect:first) echo '` + first + `|2026-11-28T07:56:10.02Z' ;;
 container:inspect:second) echo '` + second + `|0001-01-01T00:00:00Z' ;;
 image:inspect:` + first + `) echo '{"Os":"linux","Architecture":"amd64"}' ;;
 image:inspect:` + second + `) echo '{"Os":"linux","Architecture":"arm64","Variant":"v8"}' ;;
 *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	const tag = "registry.example/workflow:latest"
	for _, expected := range []struct{ container, image, platform string }{
		{"first", first, "linux/amd64"}, {"second", second, "linux/arm64/v8"},
	} {
		observed, startedAt := containerObservation(context.Background(), expected.container, tag)
		if observed["status"] != "observed" || observed["requestedReference"] != tag ||
			observed["runtimeImageId"] != expected.image || observed["platform"] != expected.platform ||
			observed["registryDigest"] != nil {
			t.Fatalf("unexpected observation for %s: %+v", expected.container, observed)
		}
		if expected.container == "first" {
			want := time.Date(2026, 11, 28, 7, 56, 10, 20_000_000, time.UTC)
			if !startedAt.Equal(want) {
				t.Fatalf("Docker start = %s, want %s", startedAt, want)
			}
		} else if !startedAt.IsZero() {
			t.Fatalf("unset Docker start = %s, want zero", startedAt)
		}
	}
	unavailable, unavailableStart := containerObservation(context.Background(), "unknown", tag)
	if unavailable["status"] != "unavailable" || unavailable["requestedReference"] != tag || unavailable["runtimeImageId"] != nil || !unavailableStart.IsZero() {
		t.Fatalf("unexpected fallback: %+v", unavailable)
	}
}
