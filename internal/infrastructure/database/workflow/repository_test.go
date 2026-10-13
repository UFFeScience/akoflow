package workflow

import (
	"context"
	"database/sql"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func TestWorkflowDefinitionCreateAndFind(t *testing.T) {
	repository := setupRepository(t)
	simulation := &domain.ActivitySimulation{DurationSeconds: 1}
	definition := Definition{
		ID: "workflow", ExternalID: "external", Name: "WF", Namespace: "science",
		Types: []domain.ActivityType{{
			ID: "type", Name: "compute", Metadata: map[string]any{"kind": "cpu"},
		}},
		Version: domain.WorkflowVersion{
			ID: "version", Version: 1, DefinitionHash: "hash",
			Activities: []domain.Activity{
				{
					ID: "a", ActivityTypeID: "type", ExternalID: "A", Name: "first",
					Kind: domain.ActivityKindTask,
					Capabilities: []domain.ActivityCapability{
						domain.ActivityCapabilityReal,
						domain.ActivityCapabilitySimulation,
					},
					Command: domain.ActivityCommand{Entrypoint: "run"}, Simulation: simulation,
					Metadata: map[string]any{"x": "y"},
				},
				{
					ID: "b", ActivityTypeID: "type", ExternalID: "B", Name: "second",
					Kind:         domain.ActivityKindTask,
					Capabilities: []domain.ActivityCapability{domain.ActivityCapabilitySimulation},
					Simulation:   simulation,
				},
				{
					ID: "c", ActivityTypeID: "type", ExternalID: "C", Name: "executable-only",
					Kind:         domain.ActivityKindTask,
					Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
					Command:      domain.ActivityCommand{Entrypoint: "true"},
				},
			},
			Dependencies: []domain.ActivityDependency{{
				ActivityID: "b", DependsOnActivityID: "a", Type: "control",
			}},
			DataDependencies: []domain.ActivityDataDependency{{
				ProducerActivityID: "a", ConsumerActivityID: "b", LogicalName: "result.bin", SizeBytes: 10_000_000_000,
			}},
		},
	}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	got, err := repository.FindVersion(context.Background(), "version")
	if err != nil || got == nil {
		t.Fatalf("find failed: %+v %v", got, err)
	}
	if got.WorkflowID != "workflow" || len(got.Activities) != 3 || len(got.Dependencies) != 1 || len(got.DataDependencies) != 1 || got.DataDependencies[0].SizeBytes != 10_000_000_000 || got.Activities[0].Metadata["x"] != "y" || got.Activities[2].Simulation != nil || got.Activities[2].Service != nil {
		t.Fatalf("unexpected workflow: %+v", got)
	}
	missing, err := repository.FindVersion(context.Background(), "missing")
	if err != nil || missing != nil {
		t.Fatal("missing version must return nil")
	}
}

func TestWorkflowDefinitionDuplicateFails(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{ID: "same", Version: domain.WorkflowVersion{ID: "v"}}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(context.Background(), definition); err == nil {
		t.Fatal("duplicate definition must fail")
	}
}

func TestWorkflowListAndFindHydrateLatestVersionAndTypes(t *testing.T) {
	repository := setupRepository(t)
	service := &domain.ServiceSpec{Ports: []int{8080}, HealthCheck: "/health", KeepAlive: true}
	definition := Definition{
		ID: "workflow", ExternalID: "external", Name: "Scientific workflow", Namespace: "science",
		Types: []domain.ActivityType{{ID: "type", Name: "Compute", Application: "simulation", DefaultImage: "ubuntu", CPUIntensity: 1, MemoryIntensity: 2, IOIntensity: 3, NetworkIntensity: 4, Metadata: map[string]any{"category": "science"}}},
		Version: domain.WorkflowVersion{ID: "version", WorkflowID: "workflow", Version: 1, DefinitionHash: "hash", Activities: []domain.Activity{{
			ID: "activity", WorkflowVersionID: "version", ActivityTypeID: "type", ExternalID: "activity", Name: "Service", Kind: domain.ActivityKindService,
			Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal}, Command: domain.ActivityCommand{Image: "ubuntu", Entrypoint: "server"}, Resources: domain.ActivityResources{CPU: 2, MemoryBytes: 1024}, Service: service, Policy: domain.ActivityPolicy{TimeoutSeconds: 30}, Priority: 4,
		}}},
	}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	found, err := repository.Find(context.Background(), definition.ID)
	if err != nil || found == nil || found.Version.ID != "version" || len(found.Types) != 1 || found.Types[0].Metadata["category"] != "science" || found.Version.Activities[0].Service == nil || found.Version.Activities[0].Service.Ports[0] != 8080 {
		t.Fatalf("Find() = %#v, %v", found, err)
	}
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 1 || values[0].Name != definition.Name {
		t.Fatalf("List() = %#v, %v", values, err)
	}
	missing, err := repository.Find(context.Background(), "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing = %#v, %v", missing, err)
	}
	if value := nullableJSON(nil, []byte(`{}`)); value != nil {
		t.Fatalf("nullable nil = %#v", value)
	}
	if value := nullableJSON(service, []byte(`{"ports":[8080]}`)); value != `{"ports":[8080]}` {
		t.Fatalf("nullable value = %#v", value)
	}
}

func TestWorkflowCreateRejectsInvalidActivityTransactionally(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{ID: "invalid", Name: "Invalid", Version: domain.WorkflowVersion{ID: "invalid-v1", Version: 1, Activities: []domain.Activity{{ID: "activity"}}}}
	if err := repository.Create(context.Background(), definition); err == nil {
		t.Fatal("expected activity validation error")
	}
	var count int
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM workflow_definitions WHERE id='invalid'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("transaction was not rolled back: count=%d err=%v", count, err)
	}
}
