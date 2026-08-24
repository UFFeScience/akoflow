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
			},
			Dependencies: []domain.ActivityDependency{{
				ActivityID: "b", DependsOnActivityID: "a", Type: "control",
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
	if got.WorkflowID != "workflow" || len(got.Activities) != 2 || len(got.Dependencies) != 1 || got.Activities[0].Metadata["x"] != "y" {
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
