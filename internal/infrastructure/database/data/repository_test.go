package data

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"

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
	seedCatalogParents(t, db)
	return New(db)
}

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

func TestArtifactBuildCatalogLifecycle(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	digest := "sha256:" + strings.Repeat("a", 64)
	version := domain.ArtifactVersion{ID: "artifact-version", ArtifactID: "tool", Version: "1", Scope: domain.CatalogScope("system")}
	if err := repository.RegisterArtifactVersion(ctx, version); err != nil {
		t.Fatal(err)
	}
	contextArtifact := domain.BuildContextArtifact{Digest: "sha256:" + strings.Repeat("b", 64), StorageURI: "artifact://contexts/value", SizeBytes: 20, MediaType: "application/gzip"}
	if err := repository.SaveBuildContext(ctx, contextArtifact); err != nil {
		t.Fatal(err)
	}
	foundContext, err := repository.FindBuildContext(ctx, contextArtifact.Digest)
	if err != nil || foundContext == nil || foundContext.StorageURI != contextArtifact.StorageURI {
		t.Fatalf("context = %#v, %v", foundContext, err)
	}
	build := domain.ArtifactBuild{ID: "build", ArtifactVersionID: version.ID, SourceType: "docker-image", ContextDigest: contextArtifact.Digest, RecipePath: "ubuntu:latest", RecipeDigest: digest, TargetFormat: "sif", TargetOS: "linux", TargetArchitecture: "amd64", BuildArguments: "{}", CacheKey: "cache"}
	if err = repository.SaveArtifactBuild(ctx, build); err != nil {
		t.Fatal(err)
	}
	assertSelectableArtifacts(t, repository, ctx, false)
	for name, lookup := range map[string]func(context.Context, string) (*domain.ArtifactBuild, error){"id": repository.FindArtifactBuild, "cache": repository.FindArtifactBuildByCacheKey} {
		argument := build.ID
		if name == "cache" {
			argument = build.CacheKey
		}
		found, findErr := lookup(ctx, argument)
		if findErr != nil || found == nil || found.ID != build.ID {
			t.Fatalf("%s build = %#v, %v", name, found, findErr)
		}
	}
	builds, err := repository.ListArtifactBuilds(ctx, version.ArtifactID)
	if err != nil || len(builds) != 1 {
		t.Fatalf("builds = %#v, %v", builds, err)
	}
	run := domain.BuildRun{ID: "build-run", ArtifactBuildID: build.ID, Status: "building", Logs: "starting"}
	if err = repository.SaveBuildRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Status, run.Logs = "publishing", "built"
	if err = repository.SaveBuildRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	foundRun, err := repository.FindBuildRun(ctx, run.ID)
	if err != nil || foundRun == nil || foundRun.Status != "publishing" {
		t.Fatalf("run = %#v, %v", foundRun, err)
	}
	runs, err := repository.ListBuildRuns(ctx, build.ID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %#v, %v", runs, err)
	}
	variant := domain.ArtifactVariant{ID: "variant", Digest: digest, Format: "sif", Architecture: "amd64", SizeBytes: 99}
	location := domain.ArtifactLocation{ID: "artifact-location", VariantID: variant.ID, EndpointID: "artifact-store", URI: "artifact://outputs/tool.sif", Digest: digest, Scope: domain.CatalogScope("system"), Available: true}
	if err = repository.PublishBuildOutput(ctx, run.ID, variant, location); err != nil {
		t.Fatal(err)
	}
	outputVariant, outputLocation, err := repository.FindBuildOutput(ctx, build.ID)
	if err != nil || outputVariant == nil || outputLocation == nil || outputVariant.Digest != digest {
		t.Fatalf("output = %#v %#v, %v", outputVariant, outputLocation, err)
	}
	catalogVariant, catalogLocation, err := repository.FindCatalogOutput(ctx, "tool", "1", "amd64")
	if err != nil || catalogVariant == nil || catalogLocation == nil {
		t.Fatalf("catalog output = %#v %#v, %v", catalogVariant, catalogLocation, err)
	}
	reference, err := repository.FindCatalogOCIReference(ctx, "tool", "1", "amd64")
	if err != nil || reference != "ubuntu:latest" {
		t.Fatalf("OCI reference = %q, %v", reference, err)
	}
	dockerBuild, dockerVariant, dockerLocation, err := repository.FindDockerBuildOutput(ctx, "ubuntu:latest", "amd64")
	if err != nil || dockerBuild == nil || dockerVariant == nil || dockerLocation == nil {
		t.Fatalf("docker output = %#v %#v %#v, %v", dockerBuild, dockerVariant, dockerLocation, err)
	}
	assertArtifactCatalog(t, repository, ctx)
	assertSelectableArtifacts(t, repository, ctx, true)
	locations, err := repository.ListArtifactLocations(ctx)
	if err != nil || len(locations) != 1 {
		t.Fatalf("locations = %#v, %v", locations, err)
	}
}

func assertSelectableArtifacts(t *testing.T, repository *Repository, ctx context.Context, published bool) {
	t.Helper()
	values, err := repository.ListArtifacts(ctx, true)
	if !published && (err != nil || len(values) != 0) {
		t.Fatalf("unpublished selectable artifacts = %#v, %v", values, err)
	}
	if published && (err != nil || len(values) != 1 || !slices.Equal(values[0].Formats, []string{"sif"}) || !slices.Equal(values[0].Architectures, []string{"amd64"})) {
		t.Fatalf("selectable artifacts = %#v, %v", values, err)
	}
}

func assertArtifactCatalog(t *testing.T, repository *Repository, ctx context.Context) {
	t.Helper()
	artifacts, err := repository.ListArtifacts(ctx, false)
	if err != nil || len(artifacts) != 1 || artifacts[0].ID != "tool" || !artifacts[0].Available {
		t.Fatalf("artifacts = %#v, %v", artifacts, err)
	}
}

func TestMaterializationAndTransferLifecycle(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	digest := "sha256:" + strings.Repeat("c", 64)
	if err := repository.RegisterArtifactVersion(ctx, domain.ArtifactVersion{ID: "version", ArtifactID: "artifact", Version: "1", Scope: domain.CatalogScope("system")}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.Exec(`INSERT INTO artifact_variants(id,artifact_version_id,digest,format) VALUES('variant','version',?,'sif')`, digest); err != nil {
		t.Fatal(err)
	}
	materialization := domain.ArtifactMaterialization{ID: "materialization", RunID: "run", ActivityID: "activity", VariantID: "variant", Digest: digest, ResourceID: "resource", EnvironmentID: "env", DestinationPath: "/work/tool.sif", Status: domain.MaterializationPlanned}
	if err := repository.SaveArtifactMaterialization(ctx, materialization); err != nil {
		t.Fatal(err)
	}
	materialization.Status, materialization.VerifiedDigest = domain.MaterializationCommitted, digest
	if err := repository.SaveArtifactMaterialization(ctx, materialization); err != nil {
		t.Fatal(err)
	}
	values, err := repository.ListArtifactMaterializations(ctx, "run")
	if err != nil || len(values) != 1 || values[0].Status != domain.MaterializationCommitted {
		t.Fatalf("materializations = %#v, %v", values, err)
	}
	all, err := repository.ListArtifactMaterializations(ctx, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("all materializations = %#v, %v", all, err)
	}
	transfer := domain.DataTransferRun{ID: "transfer-materialization", PlanID: "plan",
		ExecutionRunID: "run", ActivityID: "activity", Strategy: domain.TransferDirectRuntime,
		Status: domain.TransferRunning, VerifiedBlobs: []string{digest}, CompletedChunks: []int{1},
		StartedAt: 1, TransferredBytes: 50, LogicalBytes: 100, NetworkBytes: 50,
		Route: domain.TransferRoute{Strategy: domain.TransferDirectRuntime, SourceAddress: "10.0.0.1",
			TargetAddress: "10.0.0.2", Reason: "private network"}}
	if err = repository.SaveTransferRun(ctx, transfer); err != nil {
		t.Fatal(err)
	}
	transfer.Status, transfer.FinishedAt, transfer.TransferredBytes = domain.TransferCompleted, 2, 100
	if err = repository.SaveTransferRun(ctx, transfer); err != nil {
		t.Fatal(err)
	}
	found, err := repository.FindTransferRun(ctx, transfer.ID)
	if err != nil || found == nil || found.ExecutionRunID != "run" || found.ActivityID != "activity" ||
		len(found.VerifiedBlobs) != 1 || len(found.CompletedChunks) != 1 || found.TransferredBytes != 100 ||
		found.LogicalBytes != 100 || found.NetworkBytes != 50 || found.Route.TargetAddress != "10.0.0.2" {
		t.Fatalf("transfer = %#v, %v", found, err)
	}
	workspaceTransfer := domain.DataTransferRun{ID: "workspace-route", PlanID: "workspace-plan",
		ExecutionRunID: "run", ActivityID: "activity", Strategy: domain.TransferGateway,
		Status: domain.TransferCompleted}
	if err := repository.SaveTransferRun(ctx, workspaceTransfer); err != nil {
		t.Fatal(err)
	}
	chunk := domain.TransferChunkRun{TransferRunID: transfer.ID, Index: 1, Offset: 50, SizeBytes: 50, Digest: digest, Status: domain.TransferCompleted, Attempts: 2}
	if err := repository.SaveTransferChunkRun(ctx, chunk); err != nil {
		t.Fatal(err)
	}
	chunk.Attempts = 3
	if err := repository.SaveTransferChunkRun(ctx, chunk); err != nil {
		t.Fatal(err)
	}
	chunkRuns, err := repository.ListTransferChunkRuns(ctx, transfer.ID)
	if err != nil || len(chunkRuns) != 1 || chunkRuns[0].Attempts != 3 {
		t.Fatalf("chunks = %#v, %v", chunkRuns, err)
	}
	transfers, err := repository.ListArtifactTransferRuns(ctx, "run")
	if err != nil || len(transfers) != 2 || transfers[0].Status != domain.TransferCompleted {
		t.Fatalf("transfers = %#v, %v", transfers, err)
	}
}

func TestCatalogHelpersAndMissingValues(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	if err := repository.CatalogArtifacts(ctx, domain.ActivityHandle{}); err != nil {
		t.Fatal(err)
	}
	if locationMetadata(domain.DataLocationAvailable) != `{"lifetime":"storage"}` || locationMetadata(domain.DataLocationEphemeral) != `{"lifetime":"pod"}` {
		t.Fatal("location metadata mismatch")
	}
	if nullable("") != nil || nullable("value") != "value" || stableID("a", "b") != stableID("a", "b") || stableID("a") == stableID("b") {
		t.Fatal("helper mismatch")
	}
	if value, err := repository.FindTransferRun(ctx, "missing"); err != nil || value != nil {
		t.Fatalf("missing transfer = %#v, %v", value, err)
	}
	if value, err := repository.FindArtifactBuild(ctx, "missing"); err != nil || value != nil {
		t.Fatalf("missing build = %#v, %v", value, err)
	}
	if value, err := repository.FindBuildRun(ctx, "missing"); err != nil || value != nil {
		t.Fatalf("missing run = %#v, %v", value, err)
	}
	if value, err := repository.FindBuildContext(ctx, "missing"); err != nil || value != nil {
		t.Fatalf("missing context = %#v, %v", value, err)
	}
	if variant, location, err := repository.FindBuildOutput(ctx, "missing"); err != nil || variant != nil || location != nil {
		t.Fatalf("missing output = %#v %#v, %v", variant, location, err)
	}
	if reference, err := repository.FindCatalogOCIReference(ctx, "missing", "", ""); err != nil || reference != "" {
		t.Fatalf("missing OCI = %q, %v", reference, err)
	}
	if err := repository.PublishBuildOutput(ctx, "missing", domain.ArtifactVariant{}, domain.ArtifactLocation{}); err == nil {
		t.Fatal("expected invalid output error")
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
