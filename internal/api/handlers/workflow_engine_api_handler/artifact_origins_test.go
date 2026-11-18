package workflow_engine_api_handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	appbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/application/imagepreparation"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	apptransfer "github.com/UFFeScience/akoflow/internal/application/transfer"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	datadb "github.com/UFFeScience/akoflow/internal/infrastructure/database/data"
	executiondb "github.com/UFFeScience/akoflow/internal/infrastructure/database/execution"
	"github.com/UFFeScience/akoflow/internal/infrastructure/dockerhub"
	"github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/local"
	_ "github.com/mattn/go-sqlite3"
)

func originDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func freshOriginCatalog(t *testing.T) *datadb.Repository {
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
	return datadb.New(db)
}

func sourceOriginFixture(t *testing.T, image string, recipe bool) (*Handler, *domain.WorkflowDefinition) {
	t.Helper()
	catalog := freshOriginCatalog(t)
	ctx := context.Background()
	manager := appbuild.Manager{Root: t.TempDir(), Catalog: catalog}
	spec := domain.ArtifactBuild{
		ID: "build-source-only", ArtifactVersionID: "source-version", SourceType: "docker-image",
		ContextDigest: originDigest([]byte(image)), RecipeDigest: originDigest([]byte(image)),
		RecipePath: image, TargetOS: "linux", TargetArchitecture: "amd64", TargetFormat: "sif", CacheKey: "source-only",
	}
	if recipe {
		spec = uploadOriginRecipe(t, manager, spec, image)
	}
	if err := catalog.RegisterArtifactVersion(ctx, domain.ArtifactVersion{ID: spec.ArtifactVersionID, ArtifactID: "library-ubuntu", Version: "22.04", Scope: domain.CatalogScope("system")}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveArtifactBuild(ctx, spec); err != nil {
		t.Fatal(err)
	}
	definition := workflowFixture()
	definition.Version.Activities[0].Command.Executable = &domain.ExecutableReference{Source: domain.ExecutableSource{
		Type: domain.ExecutableSourceCatalog, ArtifactRef: &domainworkflow.ArtifactReference{ID: "library-ubuntu", Version: "22.04"},
	}}
	definition.Version.Activities[0].Command.Image = ""
	h := newTestHandler()
	h.data = catalog
	h.build = manager
	h.workflows = &workflowRepositoryStub{definition: &definition}
	return h, &definition
}

func uploadOriginRecipe(t *testing.T, manager appbuild.Manager, spec domain.ArtifactBuild, image string) domain.ArtifactBuild {
	t.Helper()
	recipe := "FROM " + image + "\nARG MESSAGE\nCOPY input.txt /input.txt\nRUN printf '%s' \"$MESSAGE\" > /recipe-output\n"
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tr := tar.NewWriter(gz)
	for name, body := range map[string]string{"Dockerfile": recipe, "input.txt": "portable-input"} {
		if err := tr.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	uploaded, err := manager.Upload(context.Background(), bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	spec.SourceType = "dockerfile"
	spec.ContextDigest = uploaded.Digest
	spec.RecipePath = "Dockerfile"
	spec.RecipeDigest = originDigest([]byte(recipe))
	spec.TargetFormat = "oci"
	spec.BuildArguments = `{"MESSAGE":"built-on-target"}`
	return spec
}

func exportAndImportOrigin(t *testing.T, source *Handler) (*Handler, *domain.WorkflowDefinition) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/export", nil)
	request.SetPathValue("workflowId", "source")
	response := httptest.NewRecorder()
	source.ExportWorkflow(response, request)
	if response.Code != 200 {
		t.Fatalf("export: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "build-source-only") || strings.Contains(response.Body.String(), "library-ubuntu") {
		t.Fatal("export depends on source catalog identifiers")
	}
	target := newTestHandler()
	target.data = freshOriginCatalog(t)
	target.workflows = &workflowRepositoryStub{}
	request = httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(response.Body.String()))
	request.Header.Set("Content-Type", "application/yaml")
	response = httptest.NewRecorder()
	target.CreateWorkflow(response, request)
	if response.Code != 201 {
		t.Fatalf("import: %d %s", response.Code, response.Body.String())
	}
	imported := target.workflows.(*workflowRepositoryStub).created
	artifacts, err := target.data.ListArtifacts(context.Background(), true)
	if err != nil || len(artifacts) != 0 {
		t.Fatalf("import prematurely materialized catalog: %+v %v", artifacts, err)
	}
	return target, imported
}

func TestUnbuiltArtifactExportsOriginAndImportsWithoutSourceCatalog(t *testing.T) {
	image := "docker.io/library/ubuntu@sha256:" + strings.Repeat("a", 64)
	for _, recipe := range []bool{false, true} {
		t.Run(fmt.Sprint(recipe), func(t *testing.T) {
			source, definition := sourceOriginFixture(t, image, recipe)
			_, imported := exportAndImportOrigin(t, source)
			executable := imported.Version.Activities[0].Command.Executable
			if executable.Source.Origin == nil {
				t.Fatal("origin lost during YAML round trip")
			}
			if definition.Version.Activities[0].Command.Executable.Source.ArtifactRef == nil {
				t.Fatal("export mutated persisted source")
			}
			if recipe && executable.Source.Origin.Build.Arguments["MESSAGE"] != "built-on-target" {
				t.Fatal("build arguments lost")
			}
		})
	}
}

func TestPortableOriginLinuxIntegration(t *testing.T) {
	if os.Getenv("AKOFLOW_PORTABLE_INTEGRATION") != "1" {
		t.Skip("requires an isolated BuildKit and local Docker on Linux")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	image, err := dockerhub.New().Resolve(ctx, dockerhub.Selection{Repository: "library/ubuntu", Tag: "22.04"}, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("immutable Ubuntu 22.04 origin: %s", image)
	if err := exec.CommandContext(ctx, "docker", "image", "inspect", image).Run(); err == nil {
		t.Fatal("Ubuntu origin is already cached; use an uncached test engine")
	}
	t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", image).Run() })
	for _, recipe := range []bool{false, true} {
		t.Run(fmt.Sprint(recipe), func(t *testing.T) { runPortableOriginLinux(t, ctx, image, recipe) })
	}
}

func runPortableOriginLinux(t *testing.T, ctx context.Context, image string, recipe bool) {
	source, _ := sourceOriginFixture(t, image, recipe)
	target, definition := exportAndImportOrigin(t, source)
	activity := definition.Version.Activities[0]
	activity.Command.Entrypoint = "sh"
	activity.Command.Arguments = []string{"-c", ". /etc/os-release; printf '%s' \"$VERSION_ID\" > result.txt"}
	if recipe {
		activity.Command.Arguments = []string{"-c", "cat /input.txt /recipe-output > result.txt"}
	}
	request := ports.ExecutionRequest{
		Run:             domain.ExecutionRun{ID: provider.NewID("portable-proof"), Mode: domain.ExecutionModeReal},
		Workflow:        domain.WorkflowVersion{Activities: []domain.Activity{activity}},
		Plan:            domain.SchedulePlan{Assignments: []domain.PlanAssignment{{ActivityID: activity.ID, ResourceID: "local"}}},
		Resources:       []domain.Resource{{ID: "local", Architecture: "amd64"}},
		Runtimes:        []domain.EnvironmentRuntime{{ID: "local", Mode: domain.RuntimeModeExecution, Driver: domain.RuntimeDriverLocal}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "local", RuntimeID: "local", Enabled: true}},
	}
	if err := target.resolveBuildPreparations(ctx, &request); err != nil {
		t.Fatal(err)
	}
	reproducer := &appbuild.Reproducer{Manager: appbuild.Manager{Root: t.TempDir(), Catalog: target.data}, Catalog: target.data, Runner: provider.OSCommandExecutor{}}
	coordinator := apptransfer.Coordinator{Reproducer: reproducer}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	recorder := executiondb.New(db)
	ctx = imagepreparation.WithTarget(ctx, recorder, request.Run.ID, activity.ID, 1)
	gate, err := coordinator.Prepare(ctx, activity.ID, request.PreparationRequirementsByActivity[activity.ID])
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.Ready(); err != nil {
		t.Fatal(err)
	}
	if recipe {
		t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", gate.Executable.OCIReference).Run() })
	}
	verifyOriginReuse(t, ctx, target, reproducer, coordinator, request, gate)
	observations, err := recorder.ListImagePreparation(ctx, request.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	kind := domainexecution.ImagePreparationPull
	if recipe {
		kind = domainexecution.ImagePreparationBuild
	}
	if len(observations) != 3 || observations[0].Kind != kind ||
		observations[0].FinishedAt == nil || observations[0].Status != "completed" ||
		observations[1].Kind != kind || observations[1].Status != "failed" ||
		observations[2].Kind != domainexecution.ImagePreparationCacheHit {
		t.Fatalf("unexpected persisted image observations: %+v", observations)
	}
	t.Logf("persisted %s, then verified cache reuse for activity attempt %d", kind, observations[0].Attempt)
	t.Logf("verified loaded image: %s", gate.Executable.OCIReference)

	executePortableActivity(t, ctx, request, gate, recipe)
}

func executePortableActivity(t *testing.T, ctx context.Context, request ports.ExecutionRequest, gate *domain.PreparationGate, recipe bool) {
	activity := request.Workflow.Activities[0]
	activity.Command.Image = gate.Executable.OCIReference
	adapter := local.New()
	t.Setenv("AKOFLOW_LOCAL_WORKSPACE_ROOT", t.TempDir())
	t.Setenv("AKOFLOW_LOCAL_WORKSPACE_VOLUME", "")
	handle, err := adapter.Start(ctx, domain.ActivityExecutionContext{Run: request.Run, Activity: activity, Resource: request.Resources[0], RuntimeID: "local", Preparation: gate})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", handle.ExternalID).Run() })
	handle = waitPortableActivity(t, ctx, adapter, handle)
	if handle.Status != domain.HandleCompleted {
		t.Fatalf("execution: %s %s", handle.Status, handle.Log)
	}
	if handle.Artifacts == nil || len(handle.Artifacts.Files) == 0 {
		t.Fatal("activity produced no tracked files")
	}
	expected := "22.04"
	if recipe {
		expected = "portable-inputbuilt-on-target"
	}
	root, _ := handle.Metadata["artifactObservationRoot"].(string)
	data, err := os.ReadFile(root + "/result.txt")
	if err != nil || string(data) != expected {
		t.Fatalf("output=%q err=%v", data, err)
	}
	t.Logf("actual activity output: %s; tracked files: %d", data, len(handle.Artifacts.Files))
}

func waitPortableActivity(t *testing.T, ctx context.Context, adapter *local.Adapter, handle domain.ActivityHandle) domain.ActivityHandle {
	for handle.Status == domain.HandleRunning {
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(250 * time.Millisecond)
		var err error
		handle, err = adapter.Inspect(ctx, handle)
		if err != nil {
			t.Fatal(err)
		}
	}
	return handle
}

type originUnavailableRunner struct{}

func (originUnavailableRunner) Run(context.Context, string, []string, []byte) ([]byte, error) {
	return nil, fmt.Errorf("test destination temporarily unavailable")
}

func verifyOriginReuse(t *testing.T, ctx context.Context, target *Handler, reproducer *appbuild.Reproducer, coordinator apptransfer.Coordinator, request ports.ExecutionRequest, gate *domain.PreparationGate) {
	requirement := request.PreparationRequirementsByActivity[request.Workflow.Activities[0].ID]
	failed := &appbuild.Reproducer{Manager: reproducer.Manager, Catalog: target.data, Runner: originUnavailableRunner{}}
	if _, err := failed.Reproduce(ctx, *requirement.Reproduction); err == nil {
		t.Fatal("unavailable preparation succeeded")
	}
	artifacts, err := target.data.ListArtifacts(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		if artifact.Available {
			t.Fatalf("stale image remains available: %+v", artifact)
		}
	}
	again, err := coordinator.Prepare(ctx, request.Workflow.Activities[0].ID, requirement)
	if err != nil {
		t.Fatal(err)
	}
	if again.Executable.VariantID != gate.Executable.VariantID {
		t.Fatal("image verification duplicated the existing variant")
	}
	locations, err := target.data.ListArtifactLocations(ctx)
	if err != nil || len(locations) != 1 || !locations[0].Available {
		t.Fatalf("locations=%+v err=%v", locations, err)
	}
}
