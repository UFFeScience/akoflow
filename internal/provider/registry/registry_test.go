package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type adapterFake struct{ mode domain.ExecutionMode }

func (a adapterFake) Modes() []domain.ExecutionMode { return []domain.ExecutionMode{a.mode} }
func (adapterFake) Start(context.Context, domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	return domain.ActivityHandle{}, nil
}

type catalogStub struct {
	definitions []domain.EnvironmentDefinition
}

type connectionFactoryFake struct{ adapter ports.RuntimeAdapter }

func (connectionFactoryFake) Driver() domain.RuntimeDriver { return domain.RuntimeDriverKubernetes }
func (f connectionFactoryFake) Build(runtime domain.EnvironmentRuntime, connection domain.EnvironmentConnection) (ports.RuntimeAdapter, error) {
	if runtime.Configuration["connectionId"] != connection.ID {
		return nil, fmt.Errorf("unexpected connection")
	}
	return f.adapter, nil
}

func (c catalogStub) Create(context.Context, domain.EnvironmentDefinition) error  { return nil }
func (c catalogStub) Replace(context.Context, domain.EnvironmentDefinition) error { return nil }
func (c catalogStub) Delete(context.Context, string) error                        { return nil }
func (c catalogStub) List(context.Context) ([]domain.EnvironmentDefinition, error) {
	return c.definitions, nil
}
func (c catalogStub) Find(context.Context, string) (*domain.EnvironmentDefinition, error) {
	return nil, nil
}
func (c catalogStub) UpdateStatus(context.Context, string, domain.EnvironmentStatus) error {
	return nil
}
func (c catalogStub) UpsertConnection(context.Context, domain.EnvironmentConnection) error {
	return nil
}
func (c catalogStub) ListConnections(context.Context, string) ([]domain.EnvironmentConnection, error) {
	return nil, nil
}

func TestCatalogResolverMapsPersistedRuntimeIDToDriver(t *testing.T) {
	base := New()
	adapter := &adapterFake{mode: domain.ExecutionModeReal}
	if err := base.Register("kubernetes", adapter); err != nil {
		t.Fatal(err)
	}
	resolver := NewCatalogResolver(base, catalogStub{definitions: []domain.EnvironmentDefinition{{
		Runtimes: []domain.EnvironmentRuntime{{ID: "kind-kubernetes", Driver: domain.RuntimeDriverKubernetes}},
	}}})
	resolved, err := resolver.Resolve(domain.ExecutionModeReal, "kind-kubernetes")
	if err != nil || resolved != adapter {
		t.Fatalf("resolved=%T err=%v", resolved, err)
	}
}

func TestCatalogResolverBuildsRuntimeFromItsConnection(t *testing.T) {
	base := New()
	adapter := &adapterFake{mode: domain.ExecutionModeReal}
	resolver := NewCatalogResolver(base, catalogStub{definitions: []domain.EnvironmentDefinition{{
		Connections: []domain.EnvironmentConnection{{ID: "kind", Type: domain.ConnectionKubernetes}},
		Runtimes: []domain.EnvironmentRuntime{{ID: "kind-kubernetes", Driver: domain.RuntimeDriverKubernetes,
			Configuration: map[string]any{"connectionId": "kind"}}},
	}}}, connectionFactoryFake{adapter: adapter})
	resolved, err := resolver.Resolve(domain.ExecutionModeReal, "kind-kubernetes")
	if err != nil || resolved != adapter {
		t.Fatalf("resolved=%T err=%v", resolved, err)
	}
}
func (adapterFake) Inspect(_ context.Context, h domain.ActivityHandle) (domain.ActivityHandle, error) {
	return h, nil
}
func (adapterFake) Stop(context.Context, domain.ActivityHandle) error { return nil }

func TestRuntimeRegistryResolvesExactAndFallbackAdapters(t *testing.T) {
	registry := New()
	if err := registry.Register("*", adapterFake{mode: domain.ExecutionModeSimulation}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(domain.ExecutionModeSimulation, "simgrid"); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("local", adapterFake{mode: domain.ExecutionModeReal}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(domain.ExecutionModeReal, "local"); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(domain.ExecutionModeReal, "missing"); err == nil {
		t.Fatal("missing runtime must fail")
	}
	if err := registry.Register("local", adapterFake{mode: domain.ExecutionModeReal}); err == nil {
		t.Fatal("duplicate registration must fail")
	}
}
