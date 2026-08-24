package resource

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func testRepository(t *testing.T) *Repository {
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
	seedResourceParents(t, repository)
	return repository
}

func seedResourceParents(t *testing.T, repository *Repository) {
	t.Helper()
	statements := []string{
		`INSERT INTO environments(id, name) VALUES ('environment', 'test')`,
		`INSERT INTO environment_versions(id, environment_id, version, status, network_model, interference_model, cost_model, configuration_hash) VALUES ('env', 'environment', 1, 'published', '{}', '{}', '{}', 'hash')`,
		`INSERT INTO environment_runtimes(id, environment_version_id, name, driver, mode) VALUES ('k8s', 'env', 'Kubernetes', 'kubernetes', 'execution')`,
		`INSERT INTO resources(id, environment_version_id, type, name, provider_id, schedulable) VALUES ('parent', 'env', 'cluster', 'parent', 'parent', 0)`,
		`INSERT INTO resource_runtime_bindings(resource_id, runtime_id, enabled) VALUES ('parent', 'k8s', 1)`,
	}
	for _, statement := range statements {
		if _, err := repository.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepositoryLifecycle(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	parent := "parent"
	resource := domain.Resource{
		ID: "r1", EnvironmentVersionID: "env",
		ParentResourceID: &parent, Type: domain.ResourceCloudVM,
		Name: "one", ProviderID: "provider", CPUCores: 4, CPUCapacity: 4.5,
		MemoryBytes: 1000, StorageBytes: 2000, ComputeSpeedup: 2,
		PricePerSecond: .1, Schedulable: true, Metadata: map[string]any{"zone": "a"},
	}
	if err := repository.Upsert(ctx, resource); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.Exec(`INSERT INTO resource_runtime_bindings(resource_id, runtime_id, enabled) VALUES ('r1', 'k8s', 1)`); err != nil {
		t.Fatal(err)
	}
	found, err := repository.FindByID(ctx, "r1")
	if err != nil || found == nil || found.Name != "one" || found.ParentResourceID == nil || found.Metadata["zone"] != "a" {
		t.Fatalf("unexpected resource: %+v %v", found, err)
	}
	byProvider, err := repository.FindByProviderID(ctx, "env", "provider")
	if err != nil || byProvider == nil {
		t.Fatal("provider lookup failed")
	}
	if list, err := repository.ListByRuntime(ctx, "env", "k8s"); err != nil || len(list) != 2 {
		t.Fatalf("runtime list: %v %v", list, err)
	}
	if list, err := repository.ListSchedulable(ctx, "env"); err != nil || len(list) != 1 {
		t.Fatalf("schedulable list: %v %v", list, err)
	}
	resource.Name = "updated"
	resource.Schedulable = false
	if err := repository.Upsert(ctx, resource); err != nil {
		t.Fatal(err)
	}
	found, _ = repository.FindByID(ctx, "r1")
	if found.Name != "updated" || found.Schedulable {
		t.Fatal("upsert update failed")
	}
	if missing, err := repository.FindByID(ctx, "missing"); err != nil || missing != nil {
		t.Fatal("missing resource must be nil")
	}
	if missing, err := repository.FindByProviderID(ctx, "env", "missing"); err != nil || missing != nil {
		t.Fatal("missing provider must be nil")
	}
}

func TestResourceSnapshots(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	if snapshot, err := repository.LatestSnapshot(ctx, "missing"); err != nil || snapshot != nil {
		t.Fatal("missing snapshot must be nil")
	}
	when := time.Now().UTC().Truncate(time.Second)
	if err := repository.Upsert(ctx, domain.Resource{ID: "r1", EnvironmentVersionID: "env", Type: domain.ResourceCloudVM, Name: "one", ProviderID: "r1"}); err != nil {
		t.Fatal(err)
	}
	snapshot := domain.ResourceSnapshot{
		ID: "s1", ResourceID: "r1", CapturedAt: when, Available: true,
		CPUUsed: 2, MemoryUsedBytes: 3, NetworkInBPS: 4, NetworkOutBPS: 5,
		DiskReadBPS: 6, DiskWriteBPS: 7, QueueLength: 8,
		Metadata: map[string]any{"source": "test"},
	}
	if err := repository.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	got, err := repository.LatestSnapshot(ctx, "r1")
	if err != nil || got == nil || got.ID != "s1" || got.Metadata["source"] != "test" {
		t.Fatalf("unexpected snapshot: %+v %v", got, err)
	}
}

func TestRepositoryUpsertsBindingsAndRelations(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	if err := repository.Upsert(ctx, domain.Resource{ID: "r1", EnvironmentVersionID: "env", Type: domain.ResourceHPCMachine, Name: "node", ProviderID: "node"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertRuntimeBinding(ctx, domain.ResourceRuntimeBinding{ResourceID: "r1", RuntimeID: "k8s", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertRelation(ctx, domain.ResourceRelation{EnvironmentVersionID: "env", SourceResourceID: "parent", TargetResourceID: "r1", Type: domain.ResourceRelationContains}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertRuntimeBinding(ctx, domain.ResourceRuntimeBinding{ResourceID: "r1", RuntimeID: "k8s", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	var enabled bool
	if err := repository.db.QueryRow(`SELECT enabled FROM resource_runtime_bindings WHERE resource_id='r1' AND runtime_id='k8s'`).Scan(&enabled); err != nil || enabled {
		t.Fatalf("enabled=%v err=%v", enabled, err)
	}
	var relations int
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM resource_relations WHERE target_resource_id='r1'`).Scan(&relations); err != nil || relations != 1 {
		t.Fatalf("relations=%d err=%v", relations, err)
	}
}

func TestRepositoryRejectsUnserializableMetadata(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	if err := repository.Upsert(ctx, domain.Resource{Metadata: map[string]any{"bad": make(chan int)}}); err == nil {
		t.Fatal("resource metadata must fail")
	}
	if err := repository.CreateSnapshot(ctx, domain.ResourceSnapshot{Metadata: map[string]any{"bad": make(chan int)}}); err == nil {
		t.Fatal("snapshot metadata must fail")
	}
}
