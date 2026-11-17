package build

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestRecipeRegistrationAndInspectionBeforeFirstBuild(t *testing.T) {
	manager := Manager{Root: t.TempDir()}
	origin, err := manager.RecipeOrigin(context.Background(), "FROM scratch\nCMD [\"/hello\"]\n", nil, "Dockerfile", "linux/amd64", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := inspectPortableContext(*origin.Build); err != nil {
		t.Fatal(err)
	}
	info, err := manager.InspectOrigin(context.Background(), *origin)
	if err != nil || info["available"] != false || !strings.Contains(info["dockerfile"].(string), "FROM scratch") {
		t.Fatalf("source-only inspection: %+v %v", info, err)
	}
	exported, err := manager.PortableOrigin(context.Background(), domain.ArtifactBuild{SourceType: "dockerfile", ContextDigest: origin.Build.ContextDigest, RecipePath: "Dockerfile", RecipeDigest: origin.Build.RecipeDigest, TargetOS: "linux", TargetArchitecture: "amd64"})
	if err != nil || exported.Build.ContextDigest != origin.Build.ContextDigest {
		t.Fatalf("source export: %+v %v", exported, err)
	}
}

func TestRecipeRegistrationRejectsInvalidSources(t *testing.T) {
	for _, test := range []struct {
		name, recipe, path, platform string
		arguments                    map[string]string
	}{
		{name: "unpinned base", recipe: "FROM ubuntu:latest", path: "Dockerfile", platform: "linux/amd64"},
		{name: "unsafe path", recipe: "FROM scratch", path: "../Dockerfile", platform: "linux/amd64"},
		{name: "unsupported platform", recipe: "FROM scratch", path: "Dockerfile", platform: "windows/amd64"},
		{name: "secret argument", recipe: "FROM scratch", path: "Dockerfile", platform: "linux/amd64", arguments: map[string]string{"TOKEN": "secret"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (Manager{Root: t.TempDir()}).RecipeOrigin(context.Background(), test.recipe, nil, test.path, test.platform, test.arguments); err == nil {
				t.Fatal("invalid recipe accepted")
			}
		})
	}
}

func TestCachedRecipePreflightDoesNotRequireBuilder(t *testing.T) {
	origin := portableBuildFixture(t, map[string]string{"Dockerfile": "FROM scratch"})
	runner := &reproductionRunner{}
	manager := Manager{Executor: Executor{Runner: runner}}
	observed, err := manager.CheckExecutable(context.Background(), domain.ExecutableSource{Origin: &origin})
	if err != nil || !observed {
		t.Fatalf("cached image unavailable: %v", err)
	}
	if len(runner.calls) != 1 || !strings.Contains(runner.calls[0], "docker image inspect") {
		t.Fatalf("unexpected builder call: %+v", runner.calls)
	}
}

func TestRecipeUploadAcceptsConventionalRelativeTarPaths(t *testing.T) {
	fixture := portableBuildFixture(t, map[string]string{"./Dockerfile": "FROM scratch"})
	manager := Manager{Root: t.TempDir()}
	origin, err := manager.RecipeOrigin(context.Background(), "", fixture.Build.ContextArchive, "Dockerfile", "linux/amd64", nil)
	if err != nil || origin.Build.RecipePath != "Dockerfile" {
		t.Fatalf("relative tar path rejected: %+v %v", origin, err)
	}
}
