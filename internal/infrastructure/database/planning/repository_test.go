package planning

import (
	"context"
	"database/sql"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func setup(t *testing.T) *Repository {
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
	repository := New(db)
	seedPlanParents(t, repository)
	return repository
}

func seedPlanParents(t *testing.T, repository *Repository) {
	t.Helper()
	statements := []string{
		`INSERT INTO environments(id, name) VALUES ('environment', 'test')`,
		`INSERT INTO environment_versions(id, environment_id, version, status, network_model, interference_model, cost_model, configuration_hash) VALUES ('e1', 'environment', 1, 'published', '{}', '{}', '{}', 'hash')`,
		`INSERT INTO execution_scopes(id, name) VALUES ('scope', 'test')`,
		`INSERT INTO execution_scope_environments(execution_scope_id, environment_version_id) VALUES ('scope', 'e1')`,
		`INSERT INTO workflow_definitions(id, external_id, name) VALUES ('workflow', 'workflow', 'test')`,
		`INSERT INTO workflow_versions(id, workflow_id, version, definition_hash) VALUES ('w1', 'workflow', 1, 'hash')`,
		`INSERT INTO activity_types(id, name) VALUES ('type', 'task')`,
		`INSERT INTO activity_definitions(
			id, workflow_version_id, activity_type_id, external_id, name, kind,
			capabilities, command_spec, resource_requirements, policy
		) VALUES ('task', 'w1', 'type', 'task', 'task', 'task', '{}', '{}', '{}', '{}')`,
		`INSERT INTO resources(id, environment_version_id, type, name, provider_id) VALUES ('r1', 'e1', 'local_machine', 'local', 'r1')`,
		`INSERT INTO network_topologies(id, name, version, execution_scope_id) VALUES ('network-v1', 'Network', 1, 'scope')`,
	}
	for _, statement := range statements {
		if _, err := repository.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSchedulePlanSaveAndFind(t *testing.T) {
	repository := setup(t)
	plan := domain.SchedulePlan{
		ID: "p1", WorkflowVersionID: "w1", ExecutionScopeID: "scope", NetworkTopologyID: "network-v1",
		Source: domain.PlanningSourcePlugin, Algorithm: "prism", AlgorithmVersion: "1",
		Objective: "time", DeadlineSeconds: 10, Budget: 2,
		Predicted: domain.PredictedMetrics{MakespanSeconds: 8, Cost: 1, Feasible: true},
		Assignments: []domain.PlanAssignment{{
			ID: "a1", ActivityID: "task", ResourceID: "r1", CoreID: "c", SlotID: "s",
			OrderOnResource: 1, Priority: 2, PredictedReadyAt: 1, PredictedStartAt: 2,
			PredictedFinishAt: 3, PredictedRuntimeSeconds: 1,
			PredictedTransferSeconds: .5, PredictedCost: .2,
			Metadata: map[string]any{"reason": "best"},
		}},
	}
	if err := repository.Save(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	got, err := repository.Find(context.Background(), "p1")
	if err != nil || got == nil {
		t.Fatalf("find failed: %+v %v", got, err)
	}
	if got.Algorithm != "prism" || got.Source != domain.PlanningSourcePlugin || got.NetworkTopologyID != "network-v1" || len(got.Assignments) != 1 || got.Assignments[0].PlanID != "p1" || got.Assignments[0].Metadata["reason"] != "best" {
		t.Fatalf("unexpected plan: %+v", got)
	}
	missing, err := repository.Find(context.Background(), "missing")
	if err != nil || missing != nil {
		t.Fatal("missing plan must return nil")
	}
}

func TestSchedulePlanSaveErrors(t *testing.T) {
	repository := setup(t)
	bad := domain.SchedulePlan{
		ID: "bad", WorkflowVersionID: "w1", ExecutionScopeID: "scope",
		Source: domain.PlanningSourcePlugin,
		Assignments: []domain.PlanAssignment{{
			ID: "a", ActivityID: "task", ResourceID: "r1",
			Metadata: map[string]any{"channel": make(chan int)},
		}},
	}
	if err := repository.Save(context.Background(), bad); err == nil {
		t.Fatal("unserializable assignment metadata must fail")
	}
	valid := domain.SchedulePlan{ID: "duplicate", WorkflowVersionID: "w1", ExecutionScopeID: "scope", Source: domain.PlanningSourcePlugin}
	if err := repository.Save(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(context.Background(), valid); err == nil {
		t.Fatal("duplicate plan must fail")
	}
}
