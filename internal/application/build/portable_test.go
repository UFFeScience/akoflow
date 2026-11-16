package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func portableBuildFixture(t *testing.T, files map[string]string) domain.ArtifactOrigin {
	t.Helper()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tr := tar.NewWriter(gz)
	for name, body := range files {
		if err := tr.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return domain.ArtifactOrigin{
		Platform: "linux/amd64",
		Build: &domain.PortableArtifactBuild{
			ContextArchive: data.Bytes(), ContextDigest: contentDigest(data.Bytes()),
			RecipePath: "Dockerfile", RecipeDigest: contentDigest([]byte(files["Dockerfile"])),
		},
	}
}

func TestPortableContextChecksActualBytesAndExcludesCredentials(t *testing.T) {
	for _, name := range []string{".env", ".ssh/id_rsa", "config/client.pem", "../escape"} {
		origin := portableBuildFixture(t, map[string]string{"Dockerfile": "FROM scratch", name: "sensitive"})
		if err := inspectPortableContext(*origin.Build); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	origin := portableBuildFixture(t, map[string]string{"Dockerfile": "FROM scratch"})
	if err := origin.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := inspectPortableContext(*origin.Build); err != nil {
		t.Fatal(err)
	}
	origin.Build.RecipeDigest = contentDigest([]byte("different"))
	if err := inspectPortableContext(*origin.Build); err == nil {
		t.Fatal("accepted different recipe")
	}
	origin.Build.ContextArchive = append(origin.Build.ContextArchive, byte(1))
	if err := inspectPortableContext(*origin.Build); err == nil {
		t.Fatal("accepted different context")
	}
}

func TestPortableOriginExportUsesUploadedContextWithoutHostPaths(t *testing.T) {
	ctx := context.Background()
	manager := Manager{Root: t.TempDir()}
	origin := portableBuildFixture(t, map[string]string{"Dockerfile": "FROM scratch", "result.txt": "portable"})
	if _, err := manager.Upload(ctx, bytes.NewReader(origin.Build.ContextArchive)); err != nil {
		t.Fatal(err)
	}
	spec := domain.ArtifactBuild{
		SourceType: "dockerfile", ContextDigest: origin.Build.ContextDigest, RecipePath: "Dockerfile",
		RecipeDigest: origin.Build.RecipeDigest, TargetOS: "linux", TargetArchitecture: "amd64",
		BuildArguments: `{"NAME":"portable"}`,
	}
	exported, err := manager.PortableOrigin(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(exported)
	if bytes.Contains(payload, []byte(manager.Root)) || exported.Build.Arguments["NAME"] != "portable" {
		t.Fatal(string(payload))
	}
	spec.BuildArguments = `{"API_TOKEN":"never-export"}`
	if _, err := manager.PortableOrigin(ctx, spec); err == nil {
		t.Fatal("exported secret argument")
	}
}

type reproductionRunner struct {
	calls    []string
	missing  bool
	platform string
	digest   string
	fail     bool
	root     string
}

func (r *reproductionRunner) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if name == "buildctl" {
		for _, arg := range args {
			if strings.HasPrefix(arg, "type=docker,") {
				parts := strings.Split(arg, "dest=")
				if err := os.WriteFile(parts[1], []byte("actual archive"), 0600); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	}
	if len(args) > 0 && args[0] == "load" {
		r.missing = false
		return nil, nil
	}
	if len(args) > 0 && args[0] == "pull" {
		if r.fail {
			return nil, fmt.Errorf("registry unavailable")
		}
		r.missing = false
		return nil, nil
	}
	if r.missing {
		return nil, fmt.Errorf("not present")
	}
	platform := r.platform
	if platform == "" {
		platform = "amd64"
	}
	return json.Marshal([]map[string]any{{"Id": contentDigest([]byte("image")), "Os": "linux", "Architecture": platform, "RepoDigests": []string{"ubuntu@" + r.digest}}})
}

func TestReproductionRequiresVerifiedImageAndPlatformBeforeGate(t *testing.T) {
	origin := domain.ArtifactOrigin{Image: "ubuntu@" + contentDigest([]byte("registry")), Platform: "linux/amd64"}
	for _, test := range []struct {
		name     string
		fail     bool
		platform string
		digest   string
		success  bool
	}{
		{"download and inspect", false, "amd64", strings.Split(origin.Image, "@")[1], true},
		{"offline", true, "amd64", "", false},
		{"wrong platform", false, "arm64", strings.Split(origin.Image, "@")[1], false},
		{"wrong digest", false, "amd64", contentDigest([]byte("other")), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &reproductionRunner{missing: true, fail: test.fail, platform: test.platform, digest: test.digest}
			r := Reproducer{Runner: runner}
			gate, err := r.Reproduce(context.Background(), origin)
			if test.success {
				if err != nil || gate == nil || !gate.Committed() || gate.OCIReference == "" {
					t.Fatalf("%+v %v", gate, err)
				}
			} else if err == nil || gate != nil {
				t.Fatal("unverified executable escaped gate")
			}
		})
	}
}

func TestReproductionBuildLoadsArchiveAndPassesArguments(t *testing.T) {
	origin := portableBuildFixture(t, map[string]string{"Dockerfile": "FROM scratch"})
	origin.Build.Arguments = map[string]string{"NAME": "world"}
	runner := &reproductionRunner{missing: true}
	r := Reproducer{Runner: runner, Manager: Manager{Root: t.TempDir()}}
	result, err := r.Reproduce(context.Background(), origin)
	if err != nil || !result.Committed() {
		t.Fatalf("%+v %v", result, err)
	}
	calls := strings.Join(runner.calls, "\n")
	if !strings.Contains(calls, "build-arg:NAME=world") || !strings.Contains(calls, "docker load --input") {
		t.Fatal(calls)
	}
	paths, _ := filepath.Glob(filepath.Join(r.Manager.Root, "reproduce-*"))
	if len(paths) != 0 {
		t.Fatal("attempt directory leaked")
	}
}

func TestPortableExpectedDigestCannotBeIgnored(t *testing.T) {
	origin := domain.ArtifactOrigin{
		Image: "ubuntu@" + contentDigest([]byte("registry")), Platform: "linux/amd64",
		ExpectedDigest: contentDigest([]byte("another image")),
	}
	runner := &reproductionRunner{digest: strings.Split(origin.Image, "@")[1]}
	reproducer := Reproducer{Runner: runner}
	if result, err := reproducer.Reproduce(context.Background(), origin); err == nil || result != nil {
		t.Fatal("mismatching authored checksum was ignored")
	}
}
