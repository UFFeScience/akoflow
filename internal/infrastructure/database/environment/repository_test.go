package environment

import (
	"context"
	"database/sql"
	"testing"
	"time"

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

func TestEnvironmentDefinitionCreate(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{
		Environment: domain.Environment{ID: "env", Name: "hybrid", Description: "test"},
		Version:     domain.EnvironmentVersion{ID: "v1", Version: 1, Status: domain.EnvironmentVersionPublished, NetworkModel: "real", InterferenceModel: "none", CostModel: "aws", ConfigurationHash: "hash"},
		Runtimes: []domain.EnvironmentRuntime{{ID: "k8s", Name: "Kubernetes", Driver: domain.RuntimeDriverKubernetes,
			Mode: domain.RuntimeModeExecution, Role: "cloud", Configuration: map[string]any{"region": "us"}}},
		Storages: []domain.StorageResource{{ID: "shared", Name: "shared", Type: domain.StorageNFS,
			Endpoint: "nfs:/akoflow", Shared: true, RuntimeBindings: []domain.StorageRuntimeBinding{{
				RuntimeID: "k8s", Default: true, HostPath: "/shared/akoflow"}}}},
		Resources: []domain.Resource{
			{ID: "cluster", Type: domain.ResourceCluster, Name: "cluster", ProviderID: "cluster"},
			{ID: "r1", Type: domain.ResourceCloudVM, Name: "vm", ProviderID: "provider", CPUCores: 2, CPUCapacity: 2, MemoryBytes: 1024, Schedulable: true, Metadata: map[string]any{"tier": "cloud"}},
		},
		RuntimeBindings: []domain.ResourceRuntimeBinding{
			{ResourceID: "cluster", RuntimeID: "k8s", Enabled: true},
			{ResourceID: "r1", RuntimeID: "k8s", Enabled: true},
		},
		Relations: []domain.ResourceRelation{{
			SourceResourceID: "cluster", TargetResourceID: "r1", Type: domain.ResourceRelationContains,
		}},
		Connections: []domain.EnvironmentConnection{{ID: "c1", Name: "cluster", Type: domain.ConnectionSSH, Endpoint: "login.example", Username: "user", CredentialRef: "keychain:test", Configuration: map[string]any{"port": float64(22)}}},
	}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	found, err := repository.Find(context.Background(), "env")
	if err != nil || found == nil || len(found.Relations) != 1 {
		t.Fatalf("resource relations were not loaded: %+v %v", found, err)
	}
	if err := repository.Create(context.Background(), definition); err == nil {
		t.Fatal("duplicate environment must fail")
	}
	connections, err := repository.ListConnections(context.Background(), "env")
	if err != nil || len(connections) != 1 || connections[0].CredentialRef != "keychain:test" {
		t.Fatalf("connections=%+v err=%v", connections, err)
	}
	connection := connections[0]
	connection.Endpoint = "new-login.example"
	if err := repository.UpsertConnection(context.Background(), connection); err != nil {
		t.Fatal(err)
	}
	connections, _ = repository.ListConnections(context.Background(), "env")
	if connections[0].Endpoint != "new-login.example" {
		t.Fatal("connection upsert failed")
	}
	check := domain.ConnectionCheck{ID: "check-1", ConnectionID: "c1", Status: domain.ConnectionOnline,
		Message: "reachable", LatencyMS: 3.5, CheckedAt: time.Now().UTC(), Metadata: map[string]any{"namespace": "akoflow"}}
	if err := repository.SaveConnectionCheck(context.Background(), check); err != nil {
		t.Fatal(err)
	}
	history, err := repository.ListConnectionChecks(context.Background(), "c1", 10)
	if err != nil || len(history) != 1 || history[0].Status != domain.ConnectionOnline {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	foundConnection, err := repository.FindConnection(context.Background(), "c1")
	if err != nil || foundConnection == nil || foundConnection.ID != "c1" {
		t.Fatalf("connection=%+v err=%v", foundConnection, err)
	}
	definitionWithHistory, err := repository.Find(context.Background(), "env")
	if err != nil || len(definitionWithHistory.ConnectionChecks) != 1 {
		t.Fatalf("definition checks=%+v err=%v", definitionWithHistory, err)
	}
	if err := repository.UpdateStatus(context.Background(), "env", domain.EnvironmentReady); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateStatus(context.Background(), "missing", domain.EnvironmentReady); err == nil {
		t.Fatal("updating a missing environment must fail")
	}
	storage, err := repository.FindDefaultRuntimeStorage(context.Background(), "v1", "k8s")
	if err != nil || storage.ID != "shared" || storage.RuntimeBindings[0].ContainerPath != "/akoflow/data" {
		t.Fatalf("storage=%+v err=%v", storage, err)
	}
}

func TestEnvironmentRejectsTwoDefaultStoragesForRuntime(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{
		Environment: domain.Environment{ID: "env", Name: "cluster"},
		Version: domain.EnvironmentVersion{ID: "v1", Version: 1, Status: domain.EnvironmentVersionPublished,
			NetworkModel: "real", InterferenceModel: "none", CostModel: "free"},
		Runtimes: []domain.EnvironmentRuntime{{ID: "slurm", Name: "Slurm", Driver: domain.RuntimeDriverSlurm, Mode: domain.RuntimeModeExecution}},
	}
	binding := []domain.StorageRuntimeBinding{{RuntimeID: "slurm", Default: true}}
	definition.Storages = []domain.StorageResource{
		{ID: "one", Name: "one", Type: domain.StorageLustre, RuntimeBindings: binding},
		{ID: "two", Name: "two", Type: domain.StorageNFS, RuntimeBindings: binding},
	}
	if err := repository.Create(context.Background(), definition); err == nil {
		t.Fatal("two default storages for the same runtime must fail")
	}
}

func TestDiscoveredStorageIsVisibleThroughItsRuntimeBinding(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{
		Environment: domain.Environment{ID: "env", Name: "cluster"},
		Version: domain.EnvironmentVersion{ID: "v1", Version: 1, Status: domain.EnvironmentVersionPublished,
			NetworkModel: "real", InterferenceModel: "none", CostModel: "free"},
		Runtimes: []domain.EnvironmentRuntime{{ID: "slurm", Name: "SLURM", Driver: domain.RuntimeDriverSlurm,
			Mode: domain.RuntimeModeExecution}},
	}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertDiscoveredStorage(context.Background(), domain.StorageResource{
		ID: "discovered-scratch", EnvironmentVersionID: "v1", Name: "Discovered scratch",
		Type: domain.StorageLustre, Endpoint: "/scratch", CapacityBytes: 1024,
		RuntimeBindings: []domain.StorageRuntimeBinding{{RuntimeID: "slurm", HostPath: "/scratch"}},
	}); err != nil {
		t.Fatal(err)
	}
	storages, err := repository.ListRuntimeStorages(context.Background(), "v1", "slurm")
	if err != nil || len(storages) != 1 || storages[0].ID != "discovered-scratch" {
		t.Fatalf("storages=%+v err=%v", storages, err)
	}
}

func TestDeleteEnvironmentRemovesDiscoveredInventory(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{
		Environment: domain.Environment{ID: "temporary", Name: "Temporary"},
		Version: domain.EnvironmentVersion{ID: "temporary-v1", Version: 1,
			Status: domain.EnvironmentVersionPublished, NetworkModel: "real", InterferenceModel: "none", CostModel: "free"},
		Runtimes:        []domain.EnvironmentRuntime{{ID: "ssh", Name: "SSH", Driver: domain.RuntimeDriverSSH, Mode: domain.RuntimeModeExecution}},
		Resources:       []domain.Resource{{ID: "node", Type: domain.ResourceHPCMachine, Name: "node", ProviderID: "node", Schedulable: true}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "node", RuntimeID: "ssh", Enabled: true}},
		Connections:     []domain.EnvironmentConnection{{ID: "connection", Name: "login", Type: domain.ConnectionSSH}},
	}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), definition.Environment.ID); err != nil {
		t.Fatal(err)
	}
	found, err := repository.Find(context.Background(), definition.Environment.ID)
	if err != nil || found != nil {
		t.Fatalf("environment was not deleted: %+v %v", found, err)
	}
	if err := repository.Delete(context.Background(), definition.Environment.ID); err != sql.ErrNoRows {
		t.Fatalf("missing environment error=%v", err)
	}
}

func TestReplaceEnvironmentUsesCompleteDefinition(t *testing.T) {
	repository := setupRepository(t)
	initial := Definition{
		Environment: domain.Environment{ID: "editable", Name: "Before"},
		Version:     domain.EnvironmentVersion{ID: "editable-v1", Version: 1, Status: domain.EnvironmentVersionDraft, NetworkModel: "real", InterferenceModel: "none", CostModel: "free"},
		Runtimes:    []domain.EnvironmentRuntime{{ID: "editable-local", Name: "Local", Driver: domain.RuntimeDriverLocal, Mode: domain.RuntimeModeExecution}},
		Resources:   []domain.Resource{{ID: "editable-machine", Type: domain.ResourceLocalMachine, Name: "Before machine", ProviderID: "machine", Schedulable: true}},
	}
	if err := repository.Create(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	replacement := initial
	replacement.Environment.Name = "After"
	replacement.Resources[0].Name = "After machine"
	replacement.Connections = []domain.EnvironmentConnection{{ID: "editable-ssh", Name: "SSH", Type: domain.ConnectionSSH, Endpoint: "login.example"}}
	if err := repository.Replace(context.Background(), replacement); err != nil {
		t.Fatal(err)
	}
	found, err := repository.Find(context.Background(), "editable")
	if err != nil || found == nil || found.Environment.Name != "After" || found.Resources[0].Name != "After machine" || len(found.Connections) != 1 {
		t.Fatalf("replacement=%+v error=%v", found, err)
	}
}
