package build

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type availabilityRunner struct {
	calls       []string
	cached      bool
	registryErr error
}

func (r *availabilityRunner) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if strings.Contains(call, "image inspect") && !r.cached {
		return nil, errors.New("not cached")
	}
	if strings.Contains(call, "manifest inspect") {
		return nil, r.registryErr
	}
	return nil, nil
}
func TestExecutablePreflightInspectsWithoutPullingOrBuilding(t *testing.T) {
	for _, cached := range []bool{true, false} {
		runner := &availabilityRunner{cached: cached}
		manager := Manager{Executor: Executor{Runner: runner}}
		observed, err := manager.CheckExecutable(context.Background(), domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "registry.example/image:1"})
		if err != nil || !observed {
			t.Fatalf("availability failed: %v", err)
		}
		for _, call := range runner.calls {
			if !strings.Contains(call, "inspect") {
				t.Fatalf("mutating preflight command: %s", call)
			}
		}
	}
}
func TestExecutablePreflightRejectsRegistryFailureWithoutLeakingCredentials(t *testing.T) {
	runner := &availabilityRunner{registryErr: errors.New("password=unique-secret")}
	manager := Manager{Executor: Executor{Runner: runner}}
	observed, err := manager.CheckExecutable(context.Background(), domain.ExecutableSource{Reference: "registry.example/image:1"})
	if !observed || err == nil || strings.Contains(err.Error(), "unique-secret") {
		t.Fatalf("unsafe registry failure: %v", err)
	}
}
func TestPortableRecipePreflightValidatesContextWithoutBuilding(t *testing.T) {
	runner := &availabilityRunner{}
	manager := Manager{Executor: Executor{Runner: runner}}
	origin := portableBuildFixture(t, map[string]string{"Dockerfile": "FROM scratch"})
	observed, err := manager.CheckExecutable(context.Background(), domain.ExecutableSource{Origin: &origin})
	if err != nil || observed || len(runner.calls) != 1 || runner.calls[0] != "buildctl debug workers" {
		t.Fatalf("preflight started a build: %v %+v", err, runner.calls)
	}
}
