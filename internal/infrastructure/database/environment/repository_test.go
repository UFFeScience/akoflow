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
	if _, err := repository.db.Exec(`INSERT INTO activity_types(id, name) VALUES ('activity-type', 'task')`); err != nil {
		t.Fatal(err)
	}
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
			{ID: "r1", Type: domain.ResourceCloudVM, Name: "vm", ProviderID: "provider", ExecutionTarget: domain.ExecutionTargetDirect, CPUCores: 2, CPUCapacity: 2, MemoryBytes: 1024, Schedulable: true, Metadata: map[string]any{"tier": "cloud"}},
		},
		RuntimeBindings: []domain.ResourceRuntimeBinding{
			{ResourceID: "cluster", RuntimeID: "k8s", Enabled: true},
			{ResourceID: "r1", RuntimeID: "k8s", Enabled: true},
		},
		Relations: []domain.ResourceRelation{{
			SourceResourceID: "cluster", TargetResourceID: "r1", Type: domain.ResourceRelationContains,
		}},
		Profiles: []domain.ActivityResourceProfile{{
			ID: "profile", ActivityTypeID: "activity-type", ResourceID: "r1", RuntimeSeconds: 5,
			RuntimeStdDevSeconds: .5, CPUUtilization: .8, PeakMemoryBytes: 512,
			DiskReadBytes: 10, DiskWriteBytes: 20, EnergyJoules: 3, Source: "measured",
			SampleSize: 4, ModelVersion: "1", Metadata: map[string]any{"host": "node"},
		}},
		Connections: []domain.EnvironmentConnection{{ID: "c1", Name: "cluster", Type: domain.ConnectionKubernetes, Endpoint: "https://cluster.example", Configuration: map[string]any{"namespace": "science", "bearerToken": "saved-token", "insecureSkipTlsVerify": true}}},
	}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	found, err := repository.Find(context.Background(), "env")
	if err != nil || found == nil || len(found.Relations) != 1 || len(found.Profiles) != 1 || found.Profiles[0].Metadata["host"] != "node" {
		t.Fatalf("resource relations were not loaded: %+v %v", found, err)
	}
	definitions, err := repository.List(context.Background())
	if err != nil || len(definitions) != 1 || definitions[0].Environment.ID != "env" {
		t.Fatalf("definitions=%+v err=%v", definitions, err)
	}
	if err := repository.Create(context.Background(), definition); err == nil {
		t.Fatal("duplicate environment must fail")
	}
	connections, err := repository.ListConnections(context.Background(), "env")
	if err != nil || len(connections) != 1 || connections[0].Configuration["bearerToken"] != "saved-token" || connections[0].Configuration["namespace"] != "science" {
		t.Fatalf("connections=%+v err=%v", connections, err)
	}
	allConnections, err := repository.ListAllConnections(context.Background())
	if err != nil || len(allConnections) != 1 || allConnections[0].ID != "c1" {
		t.Fatalf("all connections=%+v err=%v", allConnections, err)
	}
	missingConnection, err := repository.FindConnection(context.Background(), "missing")
	if err != nil || missingConnection != nil {
		t.Fatalf("missing connection=%+v err=%v", missingConnection, err)
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

func TestFindEnvironmentWithoutVersion(t *testing.T) {
	repository := setupRepository(t)
	if _, err := repository.db.Exec(`INSERT INTO environments(id, name, description, status) VALUES ('empty', 'Empty', '', 'draft')`); err != nil {
		t.Fatal(err)
	}
	definition, err := repository.Find(context.Background(), "empty")
	if err != nil || definition == nil || definition.Version.ID != "" || len(definition.Resources) != 0 {
		t.Fatalf("definition=%+v err=%v", definition, err)
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
	if err := repository.Replace(context.Background(), Definition{Environment: domain.Environment{ID: "missing"}}); err != sql.ErrNoRows {
		t.Fatalf("missing replacement error=%v", err)
	}
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

func TestCreateRejectsUnserializableNestedConfiguration(t *testing.T) {
	invalid := make(chan int)
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{"connection", func(d *Definition) {
			d.Connections = []domain.EnvironmentConnection{{ID: "connection", Configuration: map[string]any{"invalid": invalid}}}
		}},
		{"runtime configuration", func(d *Definition) {
			d.Runtimes = []domain.EnvironmentRuntime{{ID: "runtime", Configuration: map[string]any{"invalid": invalid}}}
		}},
		{"resource", func(d *Definition) {
			d.Resources = []domain.Resource{{ID: "resource", Metadata: map[string]any{"invalid": invalid}}}
		}},
		{"runtime binding", func(d *Definition) {
			d.RuntimeBindings = []domain.ResourceRuntimeBinding{{Configuration: map[string]any{"invalid": invalid}}}
		}},
		{"relation", func(d *Definition) {
			d.Relations = []domain.ResourceRelation{{Metadata: map[string]any{"invalid": invalid}}}
		}},
		{"storage configuration", func(d *Definition) {
			d.Storages = []domain.StorageResource{{ID: "storage", Configuration: map[string]any{"invalid": invalid}}}
		}},
		{"storage metadata", func(d *Definition) {
			d.Storages = []domain.StorageResource{{ID: "storage", Metadata: map[string]any{"invalid": invalid}}}
		}},
		{"storage binding", func(d *Definition) {
			d.Storages = []domain.StorageResource{{ID: "storage", RuntimeBindings: []domain.StorageRuntimeBinding{{Configuration: map[string]any{"invalid": invalid}}}}}
		}},
		{"profile", func(d *Definition) {
			d.Profiles = []domain.ActivityResourceProfile{{ID: "profile", Metadata: map[string]any{"invalid": invalid}}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := setupRepository(t)
			definition := Definition{Environment: domain.Environment{ID: "env", Name: "test"}, Version: domain.EnvironmentVersion{ID: "version", Version: 1}}
			test.mutate(&definition)
			if err := repository.Create(context.Background(), definition); err == nil {
				t.Fatal("serialization error expected")
			}
		})
	}
}

func TestConnectionAndDiscoveryRejectUnserializableMetadata(t *testing.T) {
	repository := setupRepository(t)
	invalid := map[string]any{"invalid": make(chan int)}
	if err := repository.UpsertConnection(context.Background(), domain.EnvironmentConnection{Configuration: invalid}); err == nil {
		t.Fatal("connection serialization error expected")
	}
	if err := repository.SaveConnectionCheck(context.Background(), domain.ConnectionCheck{Metadata: invalid}); err == nil {
		t.Fatal("check serialization error expected")
	}
	if err := repository.UpsertDiscoveredStorage(context.Background(), domain.StorageResource{Configuration: invalid}); err == nil {
		t.Fatal("storage serialization error expected")
	}
	if _, err := repository.FindDefaultRuntimeStorage(context.Background(), "missing", "missing"); err != sql.ErrNoRows {
		t.Fatalf("missing default storage error=%v", err)
	}
	checks, err := repository.ListConnectionChecks(context.Background(), "missing", 0)
	if err != nil || len(checks) != 0 {
		t.Fatalf("checks=%+v err=%v", checks, err)
	}
}

func TestDeleteRejectsEnvironmentUsedByScope(t *testing.T) {
	repository := setupRepository(t)
	definition := Definition{Environment: domain.Environment{ID: "used", Name: "Used"}, Version: domain.EnvironmentVersion{ID: "used-v1", Version: 1}}
	if err := repository.Create(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.Exec(`INSERT INTO execution_scopes(id,name) VALUES ('scope','Scope')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.Exec(`INSERT INTO execution_scope_environments(execution_scope_id,environment_version_id) VALUES ('scope','used-v1')`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), "used"); err == nil {
		t.Fatal("environment used by a scope must not be deleted")
	}
}
