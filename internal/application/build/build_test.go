package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type buildCatalogFake struct {
	contexts  map[string]domain.BuildContextArtifact
	runs      []domain.BuildRun
	published bool
	variant   *domain.ArtifactVariant
	location  *domain.ArtifactLocation
}

func (catalog *buildCatalogFake) SaveBuildRun(_ context.Context, run domain.BuildRun) error {
	catalog.runs = append(catalog.runs, run)
	return nil
}
func (catalog *buildCatalogFake) SaveBuildContext(_ context.Context, value domain.BuildContextArtifact) error {
	if catalog.contexts == nil {
		catalog.contexts = map[string]domain.BuildContextArtifact{}
	}
	catalog.contexts[value.Digest] = value
	return nil
}
func (catalog *buildCatalogFake) FindBuildContext(_ context.Context, digest string) (*domain.BuildContextArtifact, error) {
	value, ok := catalog.contexts[digest]
	if !ok {
		return nil, nil
	}
	return &value, nil
}
func (catalog *buildCatalogFake) PublishBuildOutput(_ context.Context, _ string, variant domain.ArtifactVariant, location domain.ArtifactLocation) error {
	catalog.published, catalog.variant, catalog.location = true, &variant, &location
	return nil
}

type contextResolverFake struct{ path string }

func (resolver contextResolverFake) ResolveBuildContext(context.Context, string) (string, error) {
	return resolver.path, nil
}

type buildRunnerFake struct{ calls [][]string }

func (runner *buildRunnerFake) Run(_ context.Context, _ string, arguments []string, _ []byte) ([]byte, error) {
	runner.calls = append(runner.calls, append([]string(nil), arguments...))
	output := ""
	for _, argument := range arguments {
		if strings.HasPrefix(argument, "type=oci,dest=") {
			output = strings.TrimPrefix(argument, "type=oci,dest=")
		}
	}
	if len(arguments) >= 4 && arguments[0] == "build" && arguments[1] == "--arch" {
		output = arguments[3]
	} else if len(arguments) >= 3 && arguments[0] == "build" && strings.HasSuffix(arguments[1], ".sif") {
		output = arguments[1]
	}
	if output != "" {
		if err := os.WriteFile(output, []byte("built-output"), 0o600); err != nil {
			return nil, err
		}
	}
	return []byte("builder log"), nil
}

func gzipTar(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, contents := range entries {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(contents))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestManagerUploadsResolvesAndOpensImmutableContext(t *testing.T) {
	root := t.TempDir()
	catalog := &buildCatalogFake{contexts: map[string]domain.BuildContextArtifact{}}
	manager := Manager{Root: root, MaxBytes: 1 << 20, Catalog: catalog}
	contextArtifact, err := manager.Upload(context.Background(), bytes.NewReader(gzipTar(t, map[string]string{
		"Dockerfile": "FROM scratch", "src/input.txt": "data",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(contextArtifact.Digest, "sha256:") || catalog.contexts[contextArtifact.Digest].Digest == "" {
		t.Fatalf("context=%+v", contextArtifact)
	}
	workspace, err := manager.ResolveBuildContext(context.Background(), contextArtifact.Digest)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(workspace, "src", "input.txt"))
	if err != nil || string(contents) != "data" {
		t.Fatalf("contents=%q err=%v", contents, err)
	}
	if cached, err := manager.ResolveBuildContext(context.Background(), contextArtifact.Digest); err != nil || cached != workspace {
		t.Fatalf("cached=%q err=%v", cached, err)
	}
	if _, err := manager.ResolveBuildContext(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid digest must fail")
	}
	if err := os.MkdirAll(filepath.Join(root, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outputs", "run.sif"), []byte("sif"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, name, err := manager.OpenOutput(context.Background(), "run")
	if err != nil || name != "run.sif" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	_, _ = io.ReadAll(reader)
	_ = reader.Close()
}

func TestManagerDefaultsAndStartsAsynchronousBuild(t *testing.T) {
	if got := (Manager{}).MaxUploadBytes(); got != 512<<20 {
		t.Fatalf("default upload limit=%d", got)
	}
	if got := (Manager{MaxBytes: 123}).MaxUploadBytes(); got != 123 {
		t.Fatalf("configured upload limit=%d", got)
	}
	root := t.TempDir()
	catalog, runner := &buildCatalogFake{}, &buildRunnerFake{}
	manager := Manager{Catalog: catalog, Executor: Executor{
		Catalog: catalog, Runner: runner, Apptainer: "apptainer", ArtifactStoreRoot: root,
	}}
	run, err := manager.Start(context.Background(), domain.ArtifactBuild{
		ID: "build", SourceType: "docker-image", RecipePath: "alpine", TargetFormat: "sif",
	})
	if err != nil || run.Status != "queued" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(catalog.runs) > 0 && catalog.runs[len(catalog.runs)-1].Status == "completed" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("asynchronous build did not complete: %+v", catalog.runs)
}

func TestManagerRejectsEmptyInvalidAndUnsafeArchives(t *testing.T) {
	if _, err := (Manager{}).Upload(context.Background(), strings.NewReader("value")); err == nil {
		t.Fatal("missing artifact root must fail")
	}
	manager := Manager{Root: t.TempDir(), MaxBytes: 32}
	if _, err := manager.Upload(context.Background(), bytes.NewReader(nil)); err == nil {
		t.Fatal("empty context must fail")
	}
	if _, err := manager.Upload(context.Background(), strings.NewReader("not gzip")); err == nil {
		t.Fatal("invalid gzip must fail")
	}
	if _, err := manager.Upload(context.Background(), bytes.NewReader(bytes.Repeat([]byte("x"), 33))); err == nil {
		t.Fatal("oversized context must fail")
	}
	unsafe := gzipTar(t, map[string]string{"../escape": "value"})
	if _, err := manager.Upload(context.Background(), bytes.NewReader(unsafe)); err == nil {
		t.Fatal("unsafe archive must fail")
	}
	if safeArchivePath("../escape") || !safeArchivePath("src/file") {
		t.Fatal("archive path validation is incorrect")
	}
	if _, _, err := manager.OpenOutput(context.Background(), "../run"); err == nil {
		t.Fatal("unsafe run ID must fail")
	}
}

func TestExecutorBuildsAndPublishesDockerSIF(t *testing.T) {
	root := t.TempDir()
	catalog, runner := &buildCatalogFake{}, &buildRunnerFake{}
	executor := Executor{Catalog: catalog, Runner: runner, Apptainer: "apptainer", ArtifactStoreRoot: root}
	run, err := executor.Execute(context.Background(), domain.ArtifactBuild{
		ID: "build", SourceType: "docker-image", RecipePath: "docker://alpine:3.20",
		TargetFormat: "sif", TargetArchitecture: "arm64",
	}, domain.BuildRun{ID: "run", ArtifactBuildID: "build"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" || !catalog.published || catalog.variant.Format != "sif" || catalog.variant.Architecture != "arm64" {
		t.Fatalf("run=%+v variant=%+v", run, catalog.variant)
	}
	if len(runner.calls) != 1 || runner.calls[0][2] != "arm64" {
		t.Fatalf("calls=%+v", runner.calls)
	}
}

func TestExecutorBuildsOCIContextAndReportsConfigurationFailures(t *testing.T) {
	root, contextPath := t.TempDir(), t.TempDir()
	catalog, runner := &buildCatalogFake{}, &buildRunnerFake{}
	executor := Executor{
		Catalog: catalog, Contexts: contextResolverFake{path: contextPath}, Runner: runner,
		Buildctl: "buildctl", ArtifactStoreRoot: root,
	}
	run, err := executor.Execute(context.Background(), domain.ArtifactBuild{
		ID: "build", SourceType: "dockerfile", ContextDigest: "sha256:value",
		RecipePath: "Dockerfile", TargetFormat: "oci", TargetOS: "linux", TargetArchitecture: "amd64",
	}, domain.BuildRun{ID: "run"})
	if err != nil || run.Status != "completed" || catalog.variant.Format != "oci" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	failed, err := (Executor{}).Execute(context.Background(), domain.ArtifactBuild{}, domain.BuildRun{ID: "bad"})
	if err == nil || failed.Status != "failed" {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
}

func TestExecutorConvertsOCIToSIFAndRejectsInvalidDockerSpec(t *testing.T) {
	root, contextPath := t.TempDir(), t.TempDir()
	catalog, runner := &buildCatalogFake{}, &buildRunnerFake{}
	executor := Executor{
		Catalog: catalog, Contexts: contextResolverFake{path: contextPath}, Runner: runner,
		Buildctl: "buildctl", Apptainer: "apptainer", ArtifactStoreRoot: root,
	}
	run, err := executor.Execute(context.Background(), domain.ArtifactBuild{
		ID: "build", SourceType: "dockerfile", ContextDigest: "sha256:value",
		RecipePath: "Dockerfile", TargetFormat: "sif", TargetOS: "linux", TargetArchitecture: "amd64",
	}, domain.BuildRun{ID: "converted"})
	if err != nil || run.Status != "completed" || len(runner.calls) != 2 {
		t.Fatalf("run=%+v calls=%+v err=%v", run, runner.calls, err)
	}
	failed, err := executor.Execute(context.Background(), domain.ArtifactBuild{
		ID: "bad", SourceType: "docker-image", RecipePath: "bad image", TargetFormat: "sif",
	}, domain.BuildRun{ID: "bad"})
	if err == nil || failed.Status != "failed" {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
}

type outputCatalogFake struct {
	variant  *domain.ArtifactVariant
	location *domain.ArtifactLocation
	oci      string
}

func (catalog outputCatalogFake) FindBuildOutput(context.Context, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	return catalog.variant, catalog.location, nil
}
func (catalog outputCatalogFake) FindDockerBuildOutput(context.Context, string, string) (*domain.ArtifactBuild, *domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	return &domain.ArtifactBuild{}, catalog.variant, catalog.location, nil
}
func (catalog outputCatalogFake) FindCatalogOutput(context.Context, string, string, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	return catalog.variant, catalog.location, nil
}
func (catalog outputCatalogFake) FindCatalogOCIReference(context.Context, string, string, string) (string, error) {
	return catalog.oci, nil
}

func TestOutputResolversCreateMaterializationContracts(t *testing.T) {
	variant := &domain.ArtifactVariant{ID: "variant", Digest: "sha256:abcdef", Format: "sif", SizeBytes: 12}
	location := &domain.ArtifactLocation{URI: "artifact://outputs/value.sif"}
	catalog := outputCatalogFake{variant: variant, location: location, oci: "registry/image:tag"}
	resource := domain.Resource{ID: "hpc", EnvironmentVersionID: "environment", Architecture: "x86_64"}
	requirement, err := (OutputResolver{Catalog: catalog}).Preparation(context.Background(), "build", "activity", resource, "")
	if err != nil || requirement.Artifact == nil || requirement.ArtifactTransfer == nil {
		t.Fatalf("requirement=%+v err=%v", requirement, err)
	}
	dockerRequirement, found, err := (OutputResolver{Catalog: catalog}).PreparationForDockerImage(context.Background(), "image", "activity", resource, "/target.sif")
	if err != nil || !found || dockerRequirement.Artifact.DestinationPath != "/target.sif" {
		t.Fatalf("requirement=%+v found=%v err=%v", dockerRequirement, found, err)
	}
	if _, err := PreparationForCatalog(context.Background(), catalog, "artifact", "v1", "activity", resource, ""); err != nil {
		t.Fatal(err)
	}
	if reference, err := OCIReferenceForCatalog(context.Background(), catalog, "artifact", "v1", "aarch64"); err != nil || reference != catalog.oci {
		t.Fatalf("reference=%q err=%v", reference, err)
	}
}

func TestOutputResolversReportUnavailableCatalogs(t *testing.T) {
	resource := domain.Resource{Architecture: "x64"}
	if _, err := OCIReferenceForCatalog(context.Background(), nil, "artifact", "v1", "amd64"); err == nil {
		t.Fatal("nil OCI catalog must fail")
	}
	if _, err := PreparationForCatalog(context.Background(), nil, "artifact", "v1", "activity", resource, ""); err == nil {
		t.Fatal("nil artifact catalog must fail")
	}
	if _, err := (OutputResolver{}).Preparation(context.Background(), "build", "activity", resource, ""); err == nil {
		t.Fatal("nil build catalog must fail")
	}
	if _, found, err := (OutputResolver{Catalog: outputCatalogFake{}}).PreparationForDockerImage(context.Background(), "image", "activity", resource, ""); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if _, err := (OutputResolver{Catalog: outputCatalogFake{}}).Preparation(context.Background(), "build", "activity", resource, ""); err == nil {
		t.Fatal("missing build output must fail")
	}
	if _, err := PreparationForCatalog(context.Background(), outputCatalogFake{}, "artifact", "v1", "activity", resource, ""); err == nil {
		t.Fatal("missing catalog output must fail")
	}
	if _, err := OCIReferenceForCatalog(context.Background(), outputCatalogFake{}, "artifact", "v1", "amd64"); err == nil {
		t.Fatal("missing OCI reference must fail")
	}
}
