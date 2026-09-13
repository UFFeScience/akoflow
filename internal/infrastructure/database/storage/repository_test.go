package storage

import (
	"context"
	"database/sql"
	"strings"
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
	if err = database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO environments(id,name) VALUES('environment','Environment')`,
		`INSERT INTO environment_versions(id,environment_id,version,status,network_model,interference_model,cost_model,configuration_hash) VALUES('version','environment',1,'published','{}','{}','{}','hash')`,
		`INSERT INTO storage_resources(id,environment_version_id,name,type,endpoint,capacity_bytes,shared,read_only,configuration,credential_reference,metadata) VALUES('storage','version','Results','local','/data',1000,1,0,'{"browseRoots":[{"path":"/data","label":"Data"}],"indexPolicy":{"enabled":true,"maxDepth":3}}','credential','{"owner":"lab"}')`,
	}
	for _, statement := range statements {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return New(db)
}

func TestRepositoryListsFindsAndScansStorageConfiguration(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	values, err := repository.ListEnvironmentStorages(ctx, "environment")
	if err != nil || len(values) != 1 {
		t.Fatalf("ListEnvironmentStorages() = %#v, %v", values, err)
	}
	value := values[0]
	if value.ID != "storage" || value.Type != domain.StorageLocal || len(value.BrowseRoots) != 1 || value.BrowseRoots[0].Label != "Data" || !value.IndexPolicy.Enabled || value.Metadata["owner"] != "lab" {
		t.Fatalf("storage = %#v", value)
	}
	found, err := repository.FindStorage(ctx, "storage")
	if err != nil || found == nil || found.CapacityBytes != 1000 {
		t.Fatalf("FindStorage() = %#v, %v", found, err)
	}
	missing, err := repository.FindStorage(ctx, "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing = %#v, %v", missing, err)
	}
	values, err = repository.ListEnvironmentStorages(ctx, "missing")
	if err != nil || len(values) != 0 {
		t.Fatalf("missing list = %#v, %v", values, err)
	}
}

func TestRepositoryPersistsDownloadLifecycle(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	run := domain.DownloadRun{ID: "download", StorageID: "storage", Path: "/data/result", Status: domain.DownloadQueued, Strategy: "stream", SizeBytes: 42, CreatedAt: now, UpdatedAt: now}
	if err := repository.SaveDownload(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Path, run.Status, run.URL, run.TransferredBytes, run.UpdatedAt = "/data/result.tar.gz", domain.DownloadCompleted, "http://download", 42, now.Add(time.Second)
	if err := repository.SaveDownload(ctx, run); err != nil {
		t.Fatal(err)
	}
	found, err := repository.FindDownload(ctx, run.ID)
	if err != nil || found == nil || found.Path != run.Path || found.Status != domain.DownloadCompleted || found.TransferredBytes != 42 || found.URL != run.URL {
		t.Fatalf("FindDownload() = %#v, %v", found, err)
	}
	missing, err := repository.FindDownload(ctx, "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing = %#v, %v", missing, err)
	}
}

func TestRepositoryPersistsAndListsIndexRuns(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	run := domain.IndexRun{ID: "index", StorageID: "storage", Status: "running", CreatedAt: now}
	if err := repository.SaveIndexRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Status, run.IndexedEntries, run.FinishedAt = "completed", 7, now.Add(time.Second)
	if err := repository.SaveIndexRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	values, err := repository.ListIndexRuns(ctx, "storage")
	if err != nil || len(values) != 1 || values[0].Status != "completed" || values[0].IndexedEntries != 7 || values[0].FinishedAt.IsZero() {
		t.Fatalf("ListIndexRuns() = %#v, %v", values, err)
	}
}

func TestRepositoryPromotesDataTransactionally(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	digest := "sha256:" + strings.Repeat("a", 64)
	entry := domain.FileEntry{Path: "/data/result.txt", SizeBytes: 123}
	if err := repository.PromoteData(ctx, "storage", entry.Path, "", "", "", "", entry, digest); err != nil {
		t.Fatal(err)
	}
	var objectID, checksum, uri string
	if err := repository.db.QueryRow(`SELECT d.id,i.checksum,l.uri FROM data_objects d JOIN data_object_instances i ON i.data_object_id=d.id JOIN data_locations l ON l.data_object_instance_id=i.id`).Scan(&objectID, &checksum, &uri); err != nil {
		t.Fatal(err)
	}
	if objectID != "data-"+strings.Repeat("a", 64) || checksum != digest || uri != "storage://storage//data/result.txt" {
		t.Fatalf("promotion = %q %q %q", objectID, checksum, uri)
	}
	if err := repository.PromoteData(ctx, "storage", entry.Path, "", "", "", "bad", entry, "md5:bad"); err == nil {
		t.Fatal("expected digest validation")
	}
}

func TestRepositoryPromotesArtifactsAndReusesMatchingEndpoint(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	digestA, digestB := "sha256:"+strings.Repeat("b", 64), "sha256:"+strings.Repeat("c", 64)
	entry := domain.FileEntry{SizeBytes: 456}
	if err := repository.PromoteArtifact(ctx, "storage", "/data/tool.sif", "tool", "Tool", "1", "environment", "environment", digestA, entry); err != nil {
		t.Fatal(err)
	}
	if err := repository.PromoteArtifact(ctx, "storage", "/data/tool-v2.sif", "tool", "Tool", "2", "environment", "environment", digestB, entry); err != nil {
		t.Fatal(err)
	}
	var versions, variants, locations, endpoints int
	for query, target := range map[string]*int{
		`SELECT COUNT(*) FROM artifact_versions`:  &versions,
		`SELECT COUNT(*) FROM artifact_variants`:  &variants,
		`SELECT COUNT(*) FROM artifact_locations`: &locations,
		`SELECT COUNT(*) FROM transfer_endpoints`: &endpoints,
	} {
		if err := repository.db.QueryRow(query).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if versions != 2 || variants != 2 || locations != 2 || endpoints != 1 {
		t.Fatalf("counts = %d/%d/%d/%d", versions, variants, locations, endpoints)
	}
	if err := repository.PromoteArtifact(ctx, "storage", "/data/bad.sif", "", "", "", "", "", "invalid", entry); err == nil {
		t.Fatal("expected digest validation")
	}
}

func TestStorageHelpers(t *testing.T) {
	digest := "sha256:" + strings.Repeat("d", 64)
	if !validSHA256(digest) || validSHA256("sha256:xyz") || !sha256Digest.MatchString(digest) {
		t.Fatal("digest validation mismatch")
	}
	if optionalReference("") != nil || optionalReference("value") != "value" {
		t.Fatal("optionalReference mismatch")
	}
	if got := artifactVersionID("id", "1", "global", ""); got != "artifact-version:id:1:global:" {
		t.Fatalf("artifactVersionID = %q", got)
	}
}
