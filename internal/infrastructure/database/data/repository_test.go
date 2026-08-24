package data

import (
	"context"
	"database/sql"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func TestCatalogArtifactsIsIdempotentAndQueryable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	seedCatalogParents(t, db)
	repository := New(db)
	handle := domain.ActivityHandle{
		RunID: "run", ActivityID: "activity", ResourceID: "resource", FinishedAt: 10,
		Metadata: map[string]any{"artifactStorageType": "pvc"},
		Artifacts: &domain.ArtifactManifest{Attempt: 1, Root: "/data/run/activity",
			Files: []domain.ArtifactObservation{{Path: "result.csv", Change: domain.ArtifactCreated,
				SizeBytes: 42, Checksum: "sha256:abc"}}},
	}
	for range 2 {
		if err := repository.CatalogArtifacts(context.Background(), handle); err != nil {
			t.Fatal(err)
		}
	}
	instances, err := repository.ListInstances(context.Background(), "run")
	if err != nil || len(instances) != 1 || instances[0].Checksum != "sha256:abc" {
		t.Fatalf("instances=%+v err=%v", instances, err)
	}
	locations, err := repository.ListLocations(context.Background(), "run")
	if err != nil || len(locations) != 1 || locations[0].Status != domain.DataLocationAvailable {
		t.Fatalf("locations=%+v err=%v", locations, err)
	}
}

func seedCatalogParents(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO environments(id, name) VALUES ('environment', 'test')`,
		`INSERT INTO environment_versions(
			id, environment_id, version, status, network_model, interference_model,
			cost_model, configuration_hash
		) VALUES ('env', 'environment', 1, 'published', '{}', '{}', '{}', 'hash')`,
		`INSERT INTO workflow_definitions(id, external_id, name) VALUES ('workflow', 'workflow', 'test')`,
		`INSERT INTO workflow_versions(id, workflow_id, version, definition_hash) VALUES ('workflow-version', 'workflow', 1, 'hash')`,
		`INSERT INTO activity_types(id, name) VALUES ('type', 'task')`,
		`INSERT INTO activity_definitions(
			id, workflow_version_id, activity_type_id, external_id, name, kind,
			capabilities, command_spec, resource_requirements, policy
		) VALUES (
			'activity', 'workflow-version', 'type', 'activity', 'activity', 'task',
			'{}', '{}', '{}', '{}'
		)`,
		`INSERT INTO resources(
			id, environment_version_id, type, name, provider_id
		) VALUES ('resource', 'env', 'kubernetes_machine', 'node', 'node')`,
		`INSERT INTO execution_scopes(id, name) VALUES ('scope', 'test')`,
		`INSERT INTO execution_scope_environments(execution_scope_id, environment_version_id) VALUES ('scope', 'env')`,
		`INSERT INTO schedule_plans(
			id, workflow_version_id, execution_scope_id, source, algorithm
		) VALUES ('plan', 'workflow-version', 'scope', 'plugin', 'test')`,
		`INSERT INTO execution_runs(id, schedule_plan_id, mode, status) VALUES ('run', 'plan', 'real', 'running')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}
