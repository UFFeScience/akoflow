package local

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
 container:inspect:first) echo '` + first + `' ;;
 container:inspect:second) echo '` + second + `' ;;
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
		observed := containerImageObservation(context.Background(), expected.container, tag)
		if observed["status"] != "observed" || observed["requestedReference"] != tag ||
			observed["runtimeImageId"] != expected.image || observed["platform"] != expected.platform ||
			observed["registryDigest"] != nil {
			t.Fatalf("unexpected observation for %s: %+v", expected.container, observed)
		}
	}
	unavailable := containerImageObservation(context.Background(), "unknown", tag)
	if unavailable["status"] != "unavailable" || unavailable["requestedReference"] != tag || unavailable["runtimeImageId"] != nil {
		t.Fatalf("unexpected fallback: %+v", unavailable)
	}
}
