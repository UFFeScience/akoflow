package build

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestPinnedSIFPullsMatchingPlatformBeforeConversion(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	tag, err := name.NewTag(strings.TrimPrefix(server.URL, "http://") + "/workflow:test")
	if err != nil {
		t.Fatal(err)
	}
	image, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{OS: "linux", Architecture: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(tag, image); err != nil {
		t.Fatal(err)
	}
	digest, err := image.Digest()
	if err != nil {
		t.Fatal(err)
	}
	pinned := tag.Context().Digest(digest.String()).Name()
	runner := &buildRunnerFake{}
	executor := Executor{Runner: runner}
	output := filepath.Join(t.TempDir(), "output.sif")
	if _, err := executor.buildPinnedSIF(context.Background(), pinned, "arm64", "apptainer", output); err == nil || !strings.Contains(err.Error(), "pull pinned image") {
		t.Fatalf("expected a pull error for a mismatched platform, got %v", err)
	}
	if len(runner.calls) > 0 {
		t.Fatal("conversion ran before platform verification")
	}
	if _, err := executor.buildPinnedSIF(context.Background(), pinned, "amd64", "apptainer", output); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || len(runner.calls[0]) != 3 || !strings.HasPrefix(runner.calls[0][2], "docker-archive:") {
		t.Fatalf("expected one archive conversion, got %v", runner.calls)
	}
}

type sifSourceRunnerFunc func(context.Context, string, []string, []byte) ([]byte, error)

func (f sifSourceRunnerFunc) Run(ctx context.Context, command string, args []string, input []byte) ([]byte, error) {
	return f(ctx, command, args, input)
}

func TestPinSIFSourcePublishesMissingQualifiedLocalTag(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	ref := strings.TrimPrefix(server.URL, "http://") + "/workflow:test"
	tag, err := name.NewTag(ref)
	if err != nil {
		t.Fatal(err)
	}
	var commands []string
	runner := sifSourceRunnerFunc(func(_ context.Context, command string, args []string, _ []byte) ([]byte, error) {
		call := command + " " + strings.Join(args, " ")
		commands = append(commands, call)
		switch call {
		case "docker image inspect " + ref:
			return nil, nil
		case "docker push " + ref:
			return nil, remote.Write(tag, empty.Image)
		default:
			t.Fatalf("unexpected command: %s", call)
			return nil, nil
		}
	})
	pinned, err := PinSIFSource(context.Background(), ref, runner)
	if err != nil || !strings.HasPrefix(pinned, tag.Context().Name()+"@sha256:") {
		t.Fatalf("pin after publication = %q, %v", pinned, err)
	}
	if len(commands) != 2 {
		t.Fatalf("expected inspection and push, got %v", commands)
	}
	reused, err := PinSIFSource(context.Background(), ref, runner)
	if err != nil || reused != pinned || len(commands) != 2 {
		t.Fatalf("registry tag should resolve without repeating push, got %q, %v, %v", reused, err, commands)
	}
}
