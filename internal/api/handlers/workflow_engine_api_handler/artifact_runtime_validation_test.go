package workflow_engine_api_handler

import (
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/operationerror"
)

func TestPortableRuntimeCompatibility(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, test := range []struct {
		name               string
		build              bool
		driver             domain.RuntimeDriver
		architecture, code string
	}{
		{"local image", false, domain.RuntimeDriverLocal, "x86_64", ""},
		{"kubernetes image", false, domain.RuntimeDriverKubernetes, "amd64", ""},
		{"architecture mismatch", false, domain.RuntimeDriverKubernetes, "arm64", "artifact_platform_mismatch"},
		{"local recipe", true, domain.RuntimeDriverLocal, "amd64", ""},
		{"kubernetes recipe", true, domain.RuntimeDriverKubernetes, "amd64", "artifact_runtime_unsupported"},
		{"slurm recipe", true, domain.RuntimeDriverSlurm, "amd64", "artifact_runtime_unsupported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			origin := &domain.ArtifactOrigin{Image: "ubuntu@" + digest, Platform: "linux/amd64"}
			if test.build {
				origin.Image = ""
				origin.Build = &domain.PortableArtifactBuild{ContextArchive: []byte("context"), ContextDigest: digest, RecipeDigest: digest, RecipePath: "Dockerfile"}
			}
			activity := domain.Activity{ID: "activity", Command: domain.ActivityCommand{Executable: &domain.ExecutableReference{Source: domain.ExecutableSource{Origin: origin}}}}
			err := validatePortableRuntime(activity, domain.Resource{Architecture: test.architecture}, test.driver)
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || operationerror.Describe(err).Code != test.code || operationerror.Describe(err).EntityID != activity.ID {
				t.Fatalf("missing actionable failure: %v", err)
			}
		})
	}
}
