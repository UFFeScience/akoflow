package workflow

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestExpansionRepositoryPersistsGraphAndDeduplicatesSourceEvent(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{
		ID: "workflow", ExternalID: "workflow", Name: "workflow",
		Types: []domain.ActivityType{{ID: "type", Name: "type"}},
		Version: domain.WorkflowVersion{
			ID: "version", WorkflowID: "workflow", Version: 1, DefinitionHash: "hash",
			Activities: []domain.Activity{{
				ID: "root", WorkflowVersionID: "version", ActivityTypeID: "type",
				ExternalID: "root", Name: "root", Kind: domain.ActivityKindTask,
				Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
				Command:      domain.ActivityCommand{Entrypoint: "true"},
			}},
		},
	}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	expansion := domain.WorkflowExpansion{
		ID: "expansion", WorkflowVersionID: "version", SourceActivityID: "root",
		SourceEventID: "event", Sequence: 1, ResultRevision: 1, Status: "applied",
		Activities: []domain.Activity{{ID: "child", WorkflowVersionID: "version", Name: "child"}},
		Dependencies: []domain.ActivityDependency{{
			ActivityID: "child", DependsOnActivityID: "root", Type: "control",
		}},
	}
	if _, err := repository.SaveExpansion(context.Background(), expansion); err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.SaveExpansion(context.Background(), expansion)
	if err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListExpansions(context.Background(), "version", "")
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != expansion.ID || len(items) != 1 || len(items[0].Activities) != 1 || len(items[0].Dependencies) != 1 {
		t.Fatalf("unexpected persisted expansion: %+v", items)
	}
}
