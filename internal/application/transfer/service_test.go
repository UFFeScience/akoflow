package transfer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	infra "github.com/UFFeScience/akoflow/internal/infrastructure/transfer"
)

type materializationCatalogStub struct {
	materializations []domain.ArtifactMaterialization
	runs             []domain.DataTransferRun
	err              error
}

type sessionFilesystem struct {
	infra.LocalFilesystem
	begins int
	ends   int
	opens  int
}

func (s *sessionFilesystem) Open(
	ctx context.Context,
	endpoint domain.TransferEndpoint,
	name string,
	offset int64,
) (io.ReadCloser, error) {
	s.opens++
	return s.LocalFilesystem.Open(ctx, endpoint, name, offset)
}

type transferProgressStub struct {
	runs   []domain.DataTransferRun
	chunks map[int]domain.TransferChunkRun
	events []domain.WorkflowOperationEvent
}

func (s *transferProgressStub) AppendWorkflowOperationEvent(_ context.Context, event domain.WorkflowOperationEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *transferProgressStub) SaveTransferRun(_ context.Context, value domain.DataTransferRun) error {
	s.runs = append(s.runs, value)
	return nil
}

func (s *transferProgressStub) SaveTransferChunkRun(_ context.Context, value domain.TransferChunkRun) error {
	if s.chunks == nil {
		s.chunks = map[int]domain.TransferChunkRun{}
	}
	s.chunks[value.Index] = value
	return nil
}

func (s *transferProgressStub) ListTransferChunkRuns(context.Context, string) ([]domain.TransferChunkRun, error) {
	values := make([]domain.TransferChunkRun, 0, len(s.chunks))
	for _, value := range s.chunks {
		values = append(values, value)
	}
	return values, nil
}

func (s *sessionFilesystem) BeginTransferSession(context.Context, domain.TransferEndpoint) error {
	s.begins++
	return nil
}

func (s *sessionFilesystem) EndTransferSession(context.Context, domain.TransferEndpoint) error {
	s.ends++
	return nil
}

func (s *materializationCatalogStub) SaveArtifactMaterialization(_ context.Context, value domain.ArtifactMaterialization) error {
	if s.err != nil {
		return s.err
	}
	s.materializations = append(s.materializations, value)
	return nil
}
func (s *materializationCatalogStub) SaveTransferRun(_ context.Context, value domain.DataTransferRun) error {
	if s.err != nil {
		return s.err
	}
	s.runs = append(s.runs, value)
	return nil
}

func TestMaterializerCommitsVerifiedBlob(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("portable artifact")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	digest := fmt.Sprintf("sha256:%x", hash[:])
	plan := domain.DataTransferPlan{ID: "transfer", Source: domain.TransferLocation{URI: "file://" + source, Path: "input"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: digest}}}
	materializer := Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}
	result, run, err := materializer.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil || !result.Committed() || run.Status != domain.TransferCompleted {
		t.Fatalf("result=%+v run=%+v err=%v", result, run, err)
	}
	if run.TransferredBytes != int64(len(content)) || run.StartedAt == 0 || run.FinishedAt < run.StartedAt {
		t.Fatalf("transfer metrics=%+v", run)
	}
	if _, err = os.Stat(filepath.Join(destination, digest)); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializerClassifiesRouteAndUsesOneConnectorSession(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("session payload")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	connector := &sessionFilesystem{}
	digest := digestOf(content)
	plan := domain.DataTransferPlan{ID: "session", Source: domain.TransferLocation{URI: "file://" + source, Path: "input"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: digest, SizeBytes: int64(len(content))}}}
	_, run, err := (Materializer{Connectors: []ports.TransferConnector{connector}}).Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil {
		t.Fatal(err)
	}
	if connector.begins != 2 || connector.ends != 2 {
		t.Fatalf("sessions begin=%d end=%d", connector.begins, connector.ends)
	}
	if run.Strategy != domain.TransferGateway || run.Route.Reason == "" || run.LogicalBytes != int64(len(content)) || run.NetworkBytes != int64(len(content)) {
		t.Fatalf("run = %#v", run)
	}
}

func TestMaterializerTransfersIntoMissingLocalWorkspaceAndRecordsRoute(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "workspace", "runs", "run-1", "consumer")
	content := []byte("output from another environment")
	if err := os.WriteFile(filepath.Join(source, "result.txt"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := digestOf(content)
	plan := domain.DataTransferPlan{
		ID: "cross-environment", ExecutionRunID: "run-1", ConsumerActivityID: "consumer",
		Source:      domain.TransferLocation{URI: "file://" + source, ResourceID: "cloud-vm", EnvironmentID: "cloud"},
		Destination: domain.TransferLocation{URI: "file://" + destination, ResourceID: "local", EnvironmentID: "desktop"},
		Blobs:       []domain.BlobDescriptor{{Digest: digest, Path: "result.txt", SizeBytes: int64(len(content))}},
	}
	_, run, err := (Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}).Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil || run.Status != domain.TransferCompleted || run.TransferredBytes != int64(len(content)) || len(run.VerifiedBlobs) != 1 {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if run.Route.SourceResourceID != "cloud-vm" || run.Route.TargetResourceID != "local" || run.Route.SourceEnvironmentID != "cloud" || run.Route.TargetEnvironmentID != "desktop" {
		t.Fatalf("route=%+v", run.Route)
	}
	if stored, err := os.ReadFile(filepath.Join(destination, "result.txt")); err != nil || string(stored) != string(content) {
		t.Fatalf("stored=%q err=%v", stored, err)
	}
}

func TestMaterializerPersistsBoundedChunkProgress(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("0123456789")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	progress := &transferProgressStub{}
	digest := digestOf(content)
	plan := domain.DataTransferPlan{ID: "chunks", ExecutionRunID: "run", ConsumerActivityID: "activity", Strategy: domain.TransferGateway, Source: domain.TransferLocation{URI: "file://" + source, Path: "input"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: digest, SizeBytes: int64(len(content))}}}
	_, run, err := (Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}, Progress: progress, ChunkSize: func(context.Context) int64 { return 4 }}).Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.chunks) != 3 || len(run.CompletedChunks) != 3 || len(progress.runs) < 4 {
		t.Fatalf("chunks=%#v run=%#v snapshots=%d", progress.chunks, run, len(progress.runs))
	}
	if len(progress.events) < 4 || progress.events[0].Phase != "started" || progress.events[len(progress.events)-1].Phase != "completed" {
		t.Fatalf("operation events=%+v", progress.events)
	}
	for index, chunk := range progress.chunks {
		if chunk.Status != domain.TransferCompleted || chunk.Index != index || chunk.Digest == "" {
			t.Fatalf("chunk %d = %#v", index, chunk)
		}
	}
}

func TestMaterializerUsesLargeChunksWithoutIncreasingRelayBuffer(t *testing.T) {
	materializer := Materializer{}
	size := materializer.chunkSize(context.Background())
	if size != 512<<20 {
		t.Fatalf("default transfer chunk size = %d", size)
	}
	if chunks := chunkCount(340_992_000, size); chunks != 1 {
		t.Fatalf("325 MiB artifact chunks = %d", chunks)
	}
	if chunks := chunkCount(600<<20, size); chunks != 2 {
		t.Fatalf("600 MiB artifact chunks = %d", chunks)
	}
}

func TestMaterializerResumesPartialAndSkipsVerifiedDestination(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("portable artifact with a resumable tail")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	digest := fmt.Sprintf("sha256:%x", hash[:])
	if err := os.WriteFile(filepath.Join(destination, digest+".partial"), content[:12], 0600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DataTransferPlan{ID: "resume", Source: domain.TransferLocation{URI: "file://" + source, Path: "input"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: digest}}}
	m := Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}
	result, run, err := m.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil || !result.Committed() || len(run.VerifiedBlobs) != 1 {
		t.Fatalf("result=%+v run=%+v err=%v", result, run, err)
	}
	got, err := os.ReadFile(filepath.Join(destination, digest))
	if err != nil || string(got) != string(content) {
		t.Fatalf("got %q, err %v", got, err)
	}
	// A second call verifies the already committed object and performs no copy.
	_, second, err := m.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil || second.Status != domain.TransferCompleted || len(second.VerifiedBlobs) != 1 {
		t.Fatalf("run=%+v err=%v", second, err)
	}
}

func TestMaterializerUsesInMemoryHitForVerifiedArtifact(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("shared executable artifact")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := digestOf(content)
	connector := &sessionFilesystem{}
	cache := &VerifiedArtifactCache{}
	materializer := Materializer{
		Connectors:        []ports.TransferConnector{connector},
		VerifiedArtifacts: cache,
	}
	plan := domain.DataTransferPlan{
		ID:          "cached-artifact",
		Source:      domain.TransferLocation{URI: "file://" + source, Path: "input"},
		Destination: domain.TransferLocation{URI: "file://" + destination},
		Blobs:       []domain.BlobDescriptor{{Digest: digest, SizeBytes: int64(len(content))}},
	}
	if _, _, err := materializer.Materialize(
		context.Background(),
		plan,
		domain.ArtifactMaterialization{Digest: digest},
	); err != nil {
		t.Fatal(err)
	}
	firstOpenCount := connector.opens
	_, hit, err := materializer.Materialize(
		context.Background(),
		plan,
		domain.ArtifactMaterialization{Digest: digest},
	)
	if err != nil || hit.TransferredBytes != 0 || len(hit.VerifiedBlobs) != 1 ||
		hit.Strategy != domain.TransferUseExisting || !strings.Contains(hit.Route.Reason, "cache hit") {
		t.Fatalf("cache hit=%+v err=%v", hit, err)
	}
	if connector.opens != firstOpenCount {
		t.Fatalf("cache hit performed remote reads: before=%d after=%d", firstOpenCount, connector.opens)
	}
}

func TestMaterializerResumesPersistedChunksAndIncrementsAttempts(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("0123456789")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := digestOf(content)
	if err := os.WriteFile(filepath.Join(destination, digest+".partial"), content[:4], 0o600); err != nil {
		t.Fatal(err)
	}
	progress := &transferProgressStub{chunks: map[int]domain.TransferChunkRun{
		0: {
			TransferRunID: "resume-chunks",
			Index:         0,
			Offset:        0,
			SizeBytes:     4,
			Status:        domain.TransferCompleted,
			Attempts:      1,
		},
		1: {
			TransferRunID: "resume-chunks",
			Index:         1,
			Offset:        4,
			SizeBytes:     4,
			Status:        domain.TransferFailed,
			Attempts:      2,
		},
	}}
	plan := domain.DataTransferPlan{
		ID:          "resume-chunks",
		Strategy:    domain.TransferGateway,
		Source:      domain.TransferLocation{URI: "file://" + source, Path: "input"},
		Destination: domain.TransferLocation{URI: "file://" + destination},
		Blobs: []domain.BlobDescriptor{{
			Digest: digest, SizeBytes: int64(len(content)),
		}},
	}
	materializer := Materializer{
		Connectors: []ports.TransferConnector{infra.LocalFilesystem{}},
		Progress:   progress,
		ChunkSize:  func(context.Context) int64 { return 4 },
	}
	_, run, err := materializer.Materialize(
		context.Background(),
		plan,
		domain.ArtifactMaterialization{Digest: digest},
	)
	if err != nil {
		t.Fatal(err)
	}
	if progress.chunks[1].Attempts != 3 {
		t.Fatalf("chunk retry attempts = %d", progress.chunks[1].Attempts)
	}
	if len(run.CompletedChunks) != 3 || run.TransferredBytes != 6 {
		t.Fatalf("resumed run = %#v", run)
	}
}

func TestMaterializerRejectsGatewayExecutionOfDestinationPull(t *testing.T) {
	plan := domain.DataTransferPlan{ID: "pull", Strategy: domain.TransferDestinationPull,
		Source: domain.TransferLocation{URI: "file:///source"}, Destination: domain.TransferLocation{URI: "file:///destination"}}
	m := Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}
	_, run, err := m.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: "sha256:abc"})
	if err == nil || run.Status != domain.TransferFailed {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}

func TestPlannerFiltersExistingBlobsAndUsesExistingLocation(t *testing.T) {
	blobs := []domain.BlobDescriptor{{Digest: "one"}, {Digest: "two"}}
	plan := (Planner{}).Plan(domain.TransferLocation{URI: "source"}, domain.TransferLocation{URI: "destination"}, blobs, map[string]bool{"one": true})
	if plan.Strategy != domain.TransferSourcePush || len(plan.Blobs) != 1 || plan.Blobs[0].Digest != "two" {
		t.Fatalf("plan = %#v", plan)
	}
	plan = (Planner{}).Plan(domain.TransferLocation{URI: "same", Path: "/data"}, domain.TransferLocation{URI: "same", Path: "/data"}, blobs, nil)
	if plan.Strategy != domain.TransferUseExisting || len(plan.Blobs) != 0 {
		t.Fatalf("existing plan = %#v", plan)
	}
}

func TestCoordinatorPreparesArtifactAndWorkspace(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	artifactContent, workspaceContent := []byte("executable"), []byte("workspace")
	artifactDigest := digestOf(artifactContent)
	workspaceDigest := digestOf(workspaceContent)
	if err := os.WriteFile(filepath.Join(source, "artifact.sif"), artifactContent, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "input.txt"), workspaceContent, 0600); err != nil {
		t.Fatal(err)
	}
	catalog := &materializationCatalogStub{}
	coordinator := Coordinator{Materializer: Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}, Catalog: catalog}
	requirement := domain.PreparationRequirement{
		Artifact:          &domain.ArtifactMaterialization{ID: "artifact", Digest: artifactDigest},
		ArtifactTransfer:  &domain.DataTransferPlan{ID: "artifact-transfer", Source: domain.TransferLocation{URI: "file://" + source, Path: "artifact.sif"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: artifactDigest, SizeBytes: int64(len(artifactContent))}}},
		Workspace:         &domain.WorkspaceMaterialization{ID: "workspace", RevisionID: "run", Missing: []domain.BlobDescriptor{{Digest: workspaceDigest, Path: "input.txt", SizeBytes: int64(len(workspaceContent))}}},
		WorkspaceTransfer: &domain.DataTransferPlan{ID: "workspace-transfer", Source: domain.TransferLocation{URI: "file://" + source}, Destination: domain.TransferLocation{URI: "file://" + destination, Path: "workspace"}, Blobs: []domain.BlobDescriptor{{Digest: workspaceDigest, Path: "input.txt", SizeBytes: int64(len(workspaceContent))}}},
	}
	gate, err := coordinator.Prepare(context.Background(), "activity", requirement)
	if err != nil {
		t.Fatalf("Prepare() = %#v, %v", gate, err)
	}
	if err = gate.Ready(); err != nil {
		t.Fatalf("gate.Ready() = %v", err)
	}
	if len(gate.TransferRuns) != 2 || len(catalog.runs) != 4 || len(catalog.materializations) != 2 {
		t.Fatalf("observations = gate:%d runs:%d materializations:%d", len(gate.TransferRuns), len(catalog.runs), len(catalog.materializations))
	}
	if gate.TransferRuns[0].ExecutionRunID != "" || gate.TransferRuns[1].ExecutionRunID != "run" || gate.TransferRuns[1].ActivityID != "activity" {
		t.Fatalf("transfer ownership = %#v", gate.TransferRuns)
	}
}

func TestCoordinatorRequiresTransferPlansAndPropagatesPersistenceFailures(t *testing.T) {
	coordinator := Coordinator{}
	if _, err := coordinator.Prepare(context.Background(), "", domain.PreparationRequirement{Artifact: &domain.ArtifactMaterialization{ID: "artifact"}}); err == nil {
		t.Fatal("expected artifact transfer error")
	}
	if _, err := coordinator.Prepare(context.Background(), "", domain.PreparationRequirement{Workspace: &domain.WorkspaceMaterialization{ID: "workspace", Missing: []domain.BlobDescriptor{{Digest: "missing"}}}}); err == nil {
		t.Fatal("expected workspace transfer error")
	}
	coordinator.Catalog = &materializationCatalogStub{err: fmt.Errorf("database")}
	plan := domain.DataTransferPlan{ID: "transfer"}
	if _, err := coordinator.Prepare(context.Background(), "", domain.PreparationRequirement{Artifact: &domain.ArtifactMaterialization{ID: "artifact"}, ArtifactTransfer: &plan}); err == nil {
		t.Fatal("expected persistence error")
	}
}

func TestMaterializerReportsMissingConnectorAndChecksumMismatch(t *testing.T) {
	plan := domain.DataTransferPlan{ID: "missing", Source: domain.TransferLocation{URI: "unknown://source"}, Destination: domain.TransferLocation{URI: "unknown://destination"}}
	if _, _, err := (Materializer{}).Materialize(context.Background(), plan, domain.ArtifactMaterialization{}); err == nil {
		t.Fatal("expected connector error")
	}
	source, destination := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "input"), []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}
	plan = domain.DataTransferPlan{ID: "checksum", Source: domain.TransferLocation{URI: "file://" + source, Path: "input"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: digestOf([]byte("expected")), SizeBytes: 5}}}
	_, run, err := (Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}).Materialize(context.Background(), plan, domain.ArtifactMaterialization{})
	if err == nil || run.Status != domain.TransferFailed {
		t.Fatalf("checksum run = %#v, %v", run, err)
	}
}

func digestOf(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", hash[:])
}
