package workflow_engine_api_handler

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/sshkey"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/token"
)

type environmentCatalogRichStub struct {
	ports.EnvironmentCatalog
	definitions       []domain.EnvironmentDefinition
	found             *domain.EnvironmentDefinition
	err               error
	replaced, deleted bool
	connection        *domain.EnvironmentConnection
}

func (s *environmentCatalogRichStub) List(context.Context) ([]domain.EnvironmentDefinition, error) {
	return s.definitions, s.err
}
func (s *environmentCatalogRichStub) Find(context.Context, string) (*domain.EnvironmentDefinition, error) {
	return s.found, s.err
}
func (s *environmentCatalogRichStub) Create(context.Context, domain.EnvironmentDefinition) error {
	return s.err
}
func (s *environmentCatalogRichStub) Replace(context.Context, domain.EnvironmentDefinition) error {
	s.replaced = true
	return s.err
}
func (s *environmentCatalogRichStub) Delete(context.Context, string) error {
	s.deleted = true
	return s.err
}
func (s *environmentCatalogRichStub) UpsertConnection(context.Context, domain.EnvironmentConnection) error {
	return s.err
}
func (s *environmentCatalogRichStub) FindConnection(context.Context, string) (*domain.EnvironmentConnection, error) {
	return s.connection, s.err
}
func (s *environmentCatalogRichStub) ListAllConnections(context.Context) ([]domain.EnvironmentConnection, error) {
	if s.connection == nil {
		return nil, s.err
	}
	return []domain.EnvironmentConnection{*s.connection}, s.err
}
func (s *environmentCatalogRichStub) SaveConnectionCheck(context.Context, domain.ConnectionCheck) error {
	return s.err
}
func (s *environmentCatalogRichStub) ListConnectionChecks(context.Context, string, int) ([]domain.ConnectionCheck, error) {
	return nil, s.err
}

type resourceInventoryRichStub struct {
	ports.ResourceInventory
	values   []domain.Resource
	found    *domain.Resource
	snapshot *domain.ResourceSnapshot
	err      error
	upserted bool
}

func (s *resourceInventoryRichStub) List(context.Context) ([]domain.Resource, error) {
	return s.values, s.err
}
func (s *resourceInventoryRichStub) FindByID(context.Context, string) (*domain.Resource, error) {
	return s.found, s.err
}
func (s *resourceInventoryRichStub) LatestSnapshot(context.Context, string) (*domain.ResourceSnapshot, error) {
	return s.snapshot, s.err
}
func (s *resourceInventoryRichStub) Upsert(context.Context, domain.Resource) error {
	s.upserted = true
	return s.err
}

type workflowStoreRichStub struct {
	ports.WorkflowStore
	values []domain.WorkflowDefinition
	found  *domain.WorkflowDefinition
	err    error
}

func (s *workflowStoreRichStub) List(context.Context) ([]domain.WorkflowDefinition, error) {
	return s.values, s.err
}
func (s *workflowStoreRichStub) Find(context.Context, string) (*domain.WorkflowDefinition, error) {
	return s.found, s.err
}

type planStoreRichStub struct {
	ports.PlanStore
	values []domain.SchedulePlan
	err    error
}

func (s *planStoreRichStub) List(context.Context) ([]domain.SchedulePlan, error) {
	return s.values, s.err
}

type scopeStoreRichStub struct {
	ports.ExecutionScopeStore
	values           []domain.ExecutionScope
	found            *domain.ExecutionScope
	err              error
	created, deleted bool
}

func (s *scopeStoreRichStub) CreateScope(context.Context, domain.ExecutionScope) error {
	s.created = true
	return s.err
}
func (s *scopeStoreRichStub) DeleteScope(context.Context, string) error {
	s.deleted = true
	return s.err
}
func (s *scopeStoreRichStub) FindScope(context.Context, string) (*domain.ExecutionScope, error) {
	return s.found, s.err
}
func (s *scopeStoreRichStub) ListScopes(context.Context) ([]domain.ExecutionScope, error) {
	return s.values, s.err
}

type instanceStoreRichStub struct {
	ports.InstanceStore
	value       *domaininstance.Instance
	preferences *domaininstance.UserPreferences
	saved       bool
	err         error
}

func (s *instanceStoreRichStub) Find(context.Context) (*domaininstance.Instance, error) {
	return s.value, s.err
}
func (s *instanceStoreRichStub) Save(_ context.Context, value domaininstance.Instance) error {
	s.value = &value
	s.saved = true
	return s.err
}
func (s *instanceStoreRichStub) FindPreferences(context.Context, string) (*domaininstance.UserPreferences, error) {
	return s.preferences, s.err
}
func (s *instanceStoreRichStub) SavePreferences(_ context.Context, value domaininstance.UserPreferences) error {
	s.preferences = &value
	s.saved = true
	return s.err
}

type dataCatalogRichStub struct {
	ports.DataCatalog
	artifacts        []domain.ExecutableArtifact
	locations        []domain.ArtifactLocation
	materializations []domain.ArtifactMaterialization
	builds           []domain.ArtifactBuild
	buildRuns        []domain.BuildRun
	build            *domain.ArtifactBuild
	cachedBuild      *domain.ArtifactBuild
	buildRun         *domain.BuildRun
	contextArtifact  *domain.BuildContextArtifact
	variant          *domain.ArtifactVariant
	location         *domain.ArtifactLocation
	oci              string
	err              error
	saved            int
	instances        []domain.DataObjectInstance
	dataLocations    []domain.DataLocation
	transferRuns     []domain.DataTransferRun
}

func (s *dataCatalogRichStub) ListArtifacts(context.Context, bool) ([]domain.ExecutableArtifact, error) {
	return s.artifacts, s.err
}
func (s *dataCatalogRichStub) ListArtifactLocations(context.Context) ([]domain.ArtifactLocation, error) {
	return s.locations, s.err
}
func (s *dataCatalogRichStub) ListArtifactMaterializations(context.Context, string) ([]domain.ArtifactMaterialization, error) {
	return s.materializations, s.err
}
func (s *dataCatalogRichStub) SaveArtifactMaterialization(context.Context, domain.ArtifactMaterialization) error {
	s.saved++
	return s.err
}
func (s *dataCatalogRichStub) SaveBuildContext(context.Context, domain.BuildContextArtifact) error {
	s.saved++
	return s.err
}
func (s *dataCatalogRichStub) FindBuildContext(context.Context, string) (*domain.BuildContextArtifact, error) {
	return s.contextArtifact, s.err
}
func (s *dataCatalogRichStub) FindArtifactBuildByCacheKey(context.Context, string) (*domain.ArtifactBuild, error) {
	return s.cachedBuild, s.err
}
func (s *dataCatalogRichStub) SaveArtifactBuild(_ context.Context, value domain.ArtifactBuild) error {
	s.saved++
	s.build = &value
	return s.err
}
func (s *dataCatalogRichStub) FindArtifactBuild(context.Context, string) (*domain.ArtifactBuild, error) {
	return s.build, s.err
}
func (s *dataCatalogRichStub) ListArtifactBuilds(context.Context, string) ([]domain.ArtifactBuild, error) {
	return s.builds, s.err
}
func (s *dataCatalogRichStub) ListBuildRuns(context.Context, string) ([]domain.BuildRun, error) {
	return s.buildRuns, s.err
}
func (s *dataCatalogRichStub) FindBuildRun(context.Context, string) (*domain.BuildRun, error) {
	return s.buildRun, s.err
}
func (s *dataCatalogRichStub) RegisterArtifactVersion(context.Context, domain.ArtifactVersion) error {
	s.saved++
	return s.err
}
func (s *dataCatalogRichStub) FindBuildOutput(context.Context, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	return s.variant, s.location, s.err
}
func (s *dataCatalogRichStub) FindDockerBuildOutput(context.Context, string, string) (*domain.ArtifactBuild, *domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	return s.build, s.variant, s.location, s.err
}
func (s *dataCatalogRichStub) FindCatalogOutput(context.Context, string, string, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	return s.variant, s.location, s.err
}
func (s *dataCatalogRichStub) FindCatalogOCIReference(context.Context, string, string, string) (string, error) {
	return s.oci, s.err
}
func (s *dataCatalogRichStub) ListInstances(context.Context, string) ([]domain.DataObjectInstance, error) {
	return s.instances, s.err
}
func (s *dataCatalogRichStub) ListLocations(context.Context, string) ([]domain.DataLocation, error) {
	return s.dataLocations, s.err
}
func (s *dataCatalogRichStub) ListArtifactTransferRuns(context.Context, string) ([]domain.DataTransferRun, error) {
	return s.transferRuns, s.err
}

type buildOrchestratorStub struct{ err error }

func (s buildOrchestratorStub) Upload(context.Context, io.Reader) (domain.BuildContextArtifact, error) {
	return domain.BuildContextArtifact{Digest: "sha256:upload", StorageURI: "artifact://upload", SizeBytes: 10}, s.err
}
func (s buildOrchestratorStub) Start(context.Context, domain.ArtifactBuild) (domain.BuildRun, error) {
	return domain.BuildRun{ID: "run"}, s.err
}
func (s buildOrchestratorStub) OpenOutput(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader("sif")), "tool.sif", s.err
}
func (s buildOrchestratorStub) MaxUploadBytes() int64 { return 1024 }

func TestCatalogAndResourceHTTPHandlers(t *testing.T) {
	environment := domain.EnvironmentDefinition{Environment: domain.Environment{ID: "environment", Name: "Science"}}
	resource := domain.Resource{ID: "resource", Name: "Node"}
	snapshot := domain.ResourceSnapshot{ID: "snapshot"}
	workflow := workflowFixture()
	plan := domain.SchedulePlan{ID: "plan"}
	scope := domain.ExecutionScope{ID: "scope", Name: "Scope"}
	environments := &environmentCatalogRichStub{definitions: []domain.EnvironmentDefinition{environment}, found: &environment}
	resources := &resourceInventoryRichStub{values: []domain.Resource{resource}, found: &resource, snapshot: &snapshot}
	workflows := &workflowStoreRichStub{values: []domain.WorkflowDefinition{workflow}, found: &workflow}
	plans := &planStoreRichStub{values: []domain.SchedulePlan{plan}}
	scopes := &scopeStoreRichStub{values: []domain.ExecutionScope{scope}, found: &scope}
	h := &Handler{environments: environments, resources: resources, workflows: workflows, plans: plans, scopes: scopes}
	tests := []struct {
		name, method, body string
		values             map[string]string
		handler            http.HandlerFunc
		status             int
		contains           string
	}{
		{"environments", http.MethodGet, "", nil, h.ListEnvironments, 200, "Science"}, {"environment", http.MethodGet, "", map[string]string{"environmentId": "environment"}, h.GetEnvironment, 200, "Science"},
		{"resources", http.MethodGet, "", nil, h.ListResources, 200, "Node"}, {"resource", http.MethodGet, "", map[string]string{"resourceId": "resource"}, h.GetResource, 200, "Node"}, {"snapshot", http.MethodGet, "", map[string]string{"resourceId": "resource"}, h.GetResourceSnapshot, 200, "snapshot"}, {"create resource", http.MethodPost, `{"id":"new","name":"New"}`, nil, h.CreateResource, 201, "New"},
		{"workflows", http.MethodGet, "", nil, h.ListWorkflows, 200, "Source"}, {"workflow", http.MethodGet, "", map[string]string{"workflowId": "source"}, h.GetWorkflow, 200, "Source"}, {"plans", http.MethodGet, "", nil, h.ListPlans, 200, "plan"},
		{"create scope", http.MethodPost, `{"id":"new-scope","name":"New scope"}`, nil, h.CreateExecutionScope, 201, "new-scope"}, {"scopes", http.MethodGet, "", nil, h.ListExecutionScopes, 200, "Scope"}, {"scope", http.MethodGet, "", map[string]string{"scopeId": "scope"}, h.GetExecutionScope, 200, "Scope"}, {"delete scope", http.MethodDelete, "", map[string]string{"scopeId": "scope"}, h.DeleteExecutionScope, 204, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := callHandler(t, test.method, "/", test.body, test.values, test.handler)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("response=%d %q", response.Code, response.Body.String())
			}
		})
	}
	if !resources.upserted || !scopes.created || !scopes.deleted {
		t.Fatalf("mutations missing")
	}
}

func TestInstancePreferencesEnvironmentReplacementAndReset(t *testing.T) {
	instance := &instanceStoreRichStub{value: &domaininstance.Instance{ID: "instance", Name: "Akoflow"}}
	definition := domain.EnvironmentDefinition{Environment: domain.Environment{ID: "environment", Name: "Environment"}}
	environments := &environmentCatalogRichStub{found: &definition}
	reset := false
	h := &Handler{instance: instance, environments: environments, factoryReset: func(context.Context) error { reset = true; return nil }}
	tests := []struct {
		name, method, body string
		values             map[string]string
		handler            http.HandlerFunc
		status             int
	}{
		{"get instance", http.MethodGet, "", nil, h.GetInstance, 200}, {"save instance", http.MethodPut, `{"id":"instance","name":"Updated"}`, nil, h.SaveInstance, 200},
		{"save preferences", http.MethodPut, `{"theme":"dark"}`, map[string]string{"clientId": "client-123"}, h.SaveUserPreferences, 200}, {"get preferences", http.MethodGet, "", map[string]string{"clientId": "client-123"}, h.GetUserPreferences, 200},
		{"replace environment", http.MethodPut, `{"environment":{"id":"environment","name":"Updated"},"version":{"id":"v1"}}`, map[string]string{"environmentId": "environment"}, h.ReplaceEnvironment, 200}, {"delete environment", http.MethodDelete, "", map[string]string{"environmentId": "environment"}, h.DeleteEnvironment, 204}, {"factory reset", http.MethodPost, "", nil, h.FactoryReset, 202},
	}
	for _, test := range tests {
		response := callHandler(t, test.method, "/", test.body, test.values, test.handler)
		if response.Code != test.status {
			t.Fatalf("%s=%d %q", test.name, response.Code, response.Body.String())
		}
	}
	if !reset || !environments.replaced || !environments.deleted || !instance.saved {
		t.Fatal("mutations missing")
	}
}

func TestArtifactAndBuildHTTPHandlers(t *testing.T) {
	build := domain.ArtifactBuild{ID: "build", ArtifactVersionID: "version", ContextDigest: "sha256:context", RecipeDigest: "sha256:recipe", CacheKey: "cache"}
	data := &dataCatalogRichStub{artifacts: []domain.ExecutableArtifact{{ID: "artifact"}}, locations: []domain.ArtifactLocation{{ID: "location"}}, materializations: []domain.ArtifactMaterialization{{ID: "materialization"}}, builds: []domain.ArtifactBuild{build}, buildRuns: []domain.BuildRun{{ID: "run"}}, build: &build, buildRun: &domain.BuildRun{ID: "run"}, contextArtifact: &domain.BuildContextArtifact{Digest: "sha256:context"}}
	h := &Handler{data: data, build: buildOrchestratorStub{}}
	tests := []struct {
		name, method, body string
		values             map[string]string
		handler            http.HandlerFunc
		status             int
		contains           string
	}{
		{"artifacts", http.MethodGet, "", nil, h.ListArtifacts, 200, "artifact"}, {"locations", http.MethodGet, "", nil, h.ListArtifactLocations, 200, "location"}, {"materializations", http.MethodGet, "", nil, h.ListArtifactMaterializations, 200, "materialization"}, {"save materialization", http.MethodPost, `{"id":"new"}`, nil, h.SaveArtifactMaterialization, 201, "new"},
		{"save context", http.MethodPost, `{"digest":"sha256:context","storageUri":"artifact://context","sizeBytes":10}`, nil, h.SaveBuildContext, 201, "sha256:context"}, {"create build", http.MethodPost, `{"id":"build-2","artifactVersionId":"version","contextDigest":"sha256:context","recipeDigest":"sha256:recipe","cacheKey":"cache-2"}`, nil, h.CreateArtifactBuild, 201, "build-2"},
		{"register docker", http.MethodPost, `{"artifactId":"tool","version":"1","image":"ubuntu:latest"}`, nil, h.RegisterDockerArtifact, 201, "docker-image"}, {"get build", http.MethodGet, "", map[string]string{"buildId": "build"}, h.GetArtifactBuild, 200, "build"}, {"list builds", http.MethodGet, "", map[string]string{"artifactId": "artifact"}, h.ListArtifactBuilds, 200, "build"}, {"list runs", http.MethodGet, "", map[string]string{"buildId": "build"}, h.ListBuildRuns, 200, "run"}, {"get run", http.MethodGet, "", map[string]string{"runId": "run"}, h.GetBuildRun, 200, "run"}, {"start", http.MethodPost, "", map[string]string{"buildId": "build"}, h.StartArtifactBuildRun, 202, "run"}, {"output", http.MethodGet, "", map[string]string{"runId": "run"}, h.StreamBuildOutput, 200, "sif"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := callHandler(t, test.method, "/", test.body, test.values, test.handler)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("response=%d %q", response.Code, response.Body.String())
			}
		})
	}
	if data.saved < 4 {
		t.Fatalf("saved=%d", data.saved)
	}
}

type connectionMonitorStub struct {
	check   domain.ConnectionCheck
	history []domain.ConnectionCheck
	err     error
}

func (s connectionMonitorStub) Check(context.Context, string) (domain.ConnectionCheck, error) {
	return s.check, s.err
}
func (s connectionMonitorStub) History(context.Context, string, int) ([]domain.ConnectionCheck, error) {
	return s.history, s.err
}

type discoveryStub struct {
	ports.EnvironmentDiscovery
	snapshots []domain.ResourceSnapshot
	err       error
}

func (s discoveryStub) DiscoverConnection(context.Context, string) ([]domain.ResourceSnapshot, error) {
	return s.snapshots, s.err
}

func TestSearchAcrossControlPlaneCatalogs(t *testing.T) {
	workflow := workflowFixture()
	workflow.Name = "Science workflow"
	environment := domain.EnvironmentDefinition{Environment: domain.Environment{ID: "science-env", Name: "Science environment", Description: "laboratory"}}
	resource := domain.Resource{ID: "science-node", Name: "Science node", ProviderID: "node"}
	plan := domain.SchedulePlan{ID: "science-plan", Algorithm: "science-scheduler"}
	scope := domain.ExecutionScope{ID: "science-scope", Name: "Science scope"}
	run := domain.ExecutionRun{ID: "science-run", Title: "Science execution"}
	data := &dataCatalogRichStub{artifacts: []domain.ExecutableArtifact{{ID: "science-artifact", Name: "Science artifact", Version: "1"}}, materializations: []domain.ArtifactMaterialization{{ID: "science-materialization", VariantID: "Science variant", RunID: run.ID}}}
	h := &Handler{workflows: &workflowStoreRichStub{values: []domain.WorkflowDefinition{workflow}}, environments: &environmentCatalogRichStub{definitions: []domain.EnvironmentDefinition{environment}}, resources: &resourceInventoryRichStub{values: []domain.Resource{resource}}, plans: &planStoreRichStub{values: []domain.SchedulePlan{plan}}, scopes: &scopeStoreRichStub{values: []domain.ExecutionScope{scope}}, executions: executionQueryStub{run: &run}, data: data}
	response := callHandler(t, http.MethodGet, "/?q=science&limit=100", "", nil, h.Search)
	if response.Code != 200 {
		t.Fatalf("search=%d %q", response.Code, response.Body.String())
	}
	for _, kind := range []string{"workflow", "environment", "resource", "plan", "scope", "execution", "artifact", "materialization"} {
		if !strings.Contains(response.Body.String(), `"type":"`+kind+`"`) {
			t.Fatalf("search lacks %s: %s", kind, response.Body.String())
		}
	}
	response = callHandler(t, http.MethodGet, "/?q=science&types=workflow&limit=1", "", nil, h.Search)
	if !strings.Contains(response.Body.String(), `"total":1`) || strings.Contains(response.Body.String(), `"type":"resource"`) {
		t.Fatalf("filtered search=%s", response.Body.String())
	}
	response = callHandler(t, http.MethodGet, "/?q=", "", nil, h.Search)
	if !strings.Contains(response.Body.String(), `"total":0`) {
		t.Fatalf("empty search=%s", response.Body.String())
	}
	if searchScore("science", "Science") != 1 || searchScore("sci", "Science") != .9 || searchScore("ence", "Science") != .7 || searchScore("none", "Science") != 0 {
		t.Fatal("search scores mismatch")
	}
	if firstNonEmpty(" ", "value") != "value" || positiveInteger("bad", 7) != 7 || positiveInteger("2", 7) != 2 {
		t.Fatal("search helpers mismatch")
	}
	types := searchTypes("workflow,invalid")
	if !types["workflow"] || types["invalid"] {
		t.Fatalf("types=%#v", types)
	}
}

func TestExecutionListingAndConnectionOperations(t *testing.T) {
	run := domain.ExecutionRun{ID: "run"}
	environments := &environmentCatalogRichStub{}
	h := &Handler{executions: executionQueryStub{run: &run}, connections: connectionMonitorStub{check: domain.ConnectionCheck{ID: "check"}, history: []domain.ConnectionCheck{{ID: "history"}}}, discovery: discoveryStub{snapshots: []domain.ResourceSnapshot{{ID: "snapshot"}}}, environments: environments, connectionTest: func(context.Context, domain.EnvironmentConnection) ports.ConnectionHealth {
		return ports.ConnectionHealth{Healthy: true, Message: "ok"}
	}}
	tests := []struct {
		name, method, target, body string
		values                     map[string]string
		handler                    http.HandlerFunc
		status                     int
		contains                   string
	}{
		{"executions", http.MethodGet, "/", "", nil, h.ListExecutions, 200, "run"}, {"executions page", http.MethodGet, "/?page=1&pageSize=10", "", nil, h.ListExecutions, 200, "pageSize"},
		{"test connection", http.MethodPost, "/", `{"id":"connection"}`, nil, h.TestEnvironmentConnection, 200, "healthy"}, {"check", http.MethodPost, "/", "", map[string]string{"connectionId": "connection"}, h.CheckEnvironmentConnection, 200, "check"}, {"history", http.MethodGet, "/?limit=5", "", map[string]string{"connectionId": "connection"}, h.ListEnvironmentConnectionHistory, 200, "history"}, {"discover", http.MethodPost, "/", "", map[string]string{"connectionId": "connection"}, h.DiscoverEnvironmentConnection, 200, "snapshot"}, {"update", http.MethodPut, "/", `{"id":"connection","environmentId":"environment"}`, map[string]string{"connectionId": "connection"}, h.UpdateEnvironmentConnection, 200, "environment"},
	}
	for _, test := range tests {
		response := callHandler(t, test.method, test.target, test.body, test.values, test.handler)
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
			t.Fatalf("%s=%d %q", test.name, response.Code, response.Body.String())
		}
	}
}

func TestDirectOCIResolutionRuntimeSelectionAndGatewayHelpers(t *testing.T) {
	workflow := domain.WorkflowVersion{Activities: []domain.Activity{
		{ID: "oci", Command: domain.ActivityCommand{Executable: &domain.ExecutableReference{Source: domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "ubuntu:latest"}}}},
		{ID: "existing", Command: domain.ActivityCommand{Image: "set", Executable: &domain.ExecutableReference{Source: domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "ignored"}}}},
	}}
	resolveDirectOCIImages(&workflow)
	if workflow.Activities[0].Command.Image != "ubuntu:latest" || workflow.Activities[1].Command.Image != "set" {
		t.Fatalf("workflow=%#v", workflow)
	}
	request := ports.ExecutionRequest{Run: domain.ExecutionRun{Mode: domain.ExecutionModeReal}, Runtimes: []domain.EnvironmentRuntime{{ID: "slurm", Driver: domain.RuntimeDriverSlurm, Mode: domain.RuntimeModeExecution}}, RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "resource", RuntimeID: "slurm", Enabled: true}}}
	if got := executionRuntimeDriver(request, domain.PlanAssignment{ResourceID: "resource"}); got != domain.RuntimeDriverSlurm {
		t.Fatalf("driver=%s", got)
	}
	if integerConfiguration(map[string]any{"a": 2, "b": 3.0, "c": "4"}, "a") != 2 || integerConfiguration(map[string]any{"b": 3.0}, "b") != 3 || integerConfiguration(map[string]any{"c": "4"}, "c") != 4 || integerConfiguration(nil, "x") != 0 {
		t.Fatal("integer config mismatch")
	}
	if normalizedContentType("Application/JSON; charset=utf-8") != "application/json" || !isYAML("text/yaml") || isYAML("text/plain") {
		t.Fatal("content type helpers mismatch")
	}
}

func TestGatewayArtifactTransferConfigurationForKubernetesAndSSH(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	requirement := func() domain.PreparationRequirement {
		return domain.PreparationRequirement{Artifact: &domain.ArtifactMaterialization{Digest: digest}, ArtifactTransfer: &domain.DataTransferPlan{}}
	}
	kubernetes := domain.EnvironmentConnection{ID: "k8s", Type: domain.ConnectionKubernetes, Configuration: map[string]any{"namespace": "science"}}
	environments := &environmentCatalogRichStub{connection: &kubernetes}
	h := &Handler{environments: environments}
	value := requirement()
	activity := domain.Activity{ID: "activity", Metadata: map[string]any{"storage": map[string]any{"claimName": "workspace", "mountPath": "/workspace"}}}
	if err := h.configureGatewayArtifactTransfer(context.Background(), &value, domain.Resource{Metadata: map[string]any{"connectionId": "k8s"}}, activity); err != nil {
		t.Fatal(err)
	}
	if value.ArtifactTransfer.Strategy != domain.TransferGateway || !strings.Contains(value.ArtifactTransfer.Destination.URI, "kubernetes://k8s/workspace/.akoflow/artifacts") || !strings.Contains(value.ArtifactTransfer.Destination.URI, "claim=workspace") {
		t.Fatalf("kubernetes requirement=%#v", value)
	}
	value = requirement()
	if err := h.configureGatewayArtifactTransfer(context.Background(), &value, domain.Resource{Metadata: map[string]any{"connectionId": "k8s"}}, domain.Activity{ID: "missing"}); err == nil {
		t.Fatal("expected PVC error")
	}
	ssh := domain.EnvironmentConnection{ID: "ssh", Type: domain.ConnectionSSH, Endpoint: "hpc.example", Username: "researcher", CredentialRef: "file:/keys/id", Configuration: map[string]any{"port": "2222", "proxyCommand": "ssh gateway", "hostKeyAlias": "hpc", "forwardAgent": true}}
	environments.connection = &ssh
	value = requirement()
	if err := h.configureGatewayArtifactTransfer(context.Background(), &value, domain.Resource{Metadata: map[string]any{"connectionId": "ssh"}}, domain.Activity{}); err != nil {
		t.Fatal(err)
	}
	if value.ArtifactTransfer.Strategy != domain.TransferSourcePush || !strings.Contains(value.ArtifactTransfer.Destination.URI, "ssh://researcher@hpc.example") || !strings.Contains(value.ArtifactTransfer.Destination.URI, "port=2222") || !strings.HasSuffix(value.Artifact.DestinationPath, digest) {
		t.Fatalf("ssh requirement=%#v", value)
	}
	value = domain.PreparationRequirement{}
	if err := h.configureGatewayArtifactTransfer(context.Background(), &value, domain.Resource{}, domain.Activity{}); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialManagementHTTPHandlers(t *testing.T) {
	directory := t.TempDir()
	sshManager := sshkey.New(directory)
	tokenManager := token.New(t.TempDir())
	h := &Handler{sshKeys: sshManager, kubernetesTokens: tokenManager}
	response := callHandler(t, http.MethodPost, "/", `{"id":"generated","comment":"test"}`, nil, h.GenerateSSHKey)
	if response.Code != 201 || !strings.Contains(response.Body.String(), "publicKey") {
		t.Fatalf("generate=%d %q", response.Code, response.Body.String())
	}
	privateKey, err := os.ReadFile(directory + "/generated")
	if err != nil {
		t.Fatal(err)
	}
	response = callHandler(t, http.MethodPost, "/", `{"id":"imported","privateKey":`+strconvQuote(string(privateKey))+`}`, nil, h.ImportSSHKey)
	if response.Code != 201 || !strings.Contains(response.Body.String(), "imported") {
		t.Fatalf("import=%d %q", response.Code, response.Body.String())
	}
	response = callHandler(t, http.MethodGet, "/", "", nil, h.ListSSHKeys)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "generated") || !strings.Contains(response.Body.String(), "imported") {
		t.Fatalf("list=%d %q", response.Code, response.Body.String())
	}
	response = callHandler(t, http.MethodPost, "/", `{"id":"cluster","token":"secret"}`, nil, h.SaveKubernetesToken)
	if response.Code != 201 || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("token=%d %q", response.Code, response.Body.String())
	}
}

func strconvQuote(value string) string { return strconv.Quote(value) }

func TestResolveBuildPreparationsForCatalogBuildAndDockerSources(t *testing.T) {
	digest := "sha256:" + strings.Repeat("b", 64)
	variant := &domain.ArtifactVariant{ID: "variant", Digest: digest, Format: "sif", SizeBytes: 20}
	location := &domain.ArtifactLocation{URI: "artifact://output.sif"}
	build := &domain.ArtifactBuild{ID: "build"}
	data := &dataCatalogRichStub{variant: variant, location: location, oci: "registry.example/tool:1", build: build}
	h := &Handler{data: data, environments: &environmentCatalogRichStub{}}
	executable := func(source domain.ExecutableSource) *domain.ExecutableReference {
		return &domain.ExecutableReference{Source: source}
	}
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{Mode: domain.ExecutionModeReal},
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{
			{ID: "catalog-k8s", Command: domain.ActivityCommand{Executable: executable(domain.ExecutableSource{Type: domain.ExecutableSourceCatalog, ArtifactRef: &domainworkflow.ArtifactReference{ID: "artifact", Version: "1"}})}},
			{ID: "build-slurm", Command: domain.ActivityCommand{Executable: executable(domain.ExecutableSource{Type: domain.ExecutableSourceType("build"), ArtifactBuildRef: "build"})}},
			{ID: "oci-slurm", Command: domain.ActivityCommand{Executable: executable(domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "ubuntu:latest"})}},
		}},
		Plan:            domain.SchedulePlan{Assignments: []domain.PlanAssignment{{ActivityID: "catalog-k8s", ResourceID: "k8s"}, {ActivityID: "build-slurm", ResourceID: "slurm"}, {ActivityID: "oci-slurm", ResourceID: "slurm"}}},
		Resources:       []domain.Resource{{ID: "k8s", Architecture: "amd64"}, {ID: "slurm", Architecture: "x86_64"}},
		Runtimes:        []domain.EnvironmentRuntime{{ID: "k8s-runtime", Driver: domain.RuntimeDriverKubernetes, Mode: domain.RuntimeModeExecution}, {ID: "slurm-runtime", Driver: domain.RuntimeDriverSlurm, Mode: domain.RuntimeModeExecution}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "k8s", RuntimeID: "k8s-runtime", Enabled: true}, {ResourceID: "slurm", RuntimeID: "slurm-runtime", Enabled: true}},
	}
	if err := h.resolveBuildPreparations(context.Background(), &request); err != nil {
		t.Fatal(err)
	}
	if request.Workflow.Activities[0].Command.Image != "registry.example/tool:1" {
		t.Fatalf("catalog image=%q", request.Workflow.Activities[0].Command.Image)
	}
	if len(request.PreparationRequirementsByActivity) != 2 || request.PreparationRequirementsByActivity["build-slurm"].Artifact == nil || request.PreparationRequirementsByActivity["oci-slurm"].Artifact == nil {
		t.Fatalf("requirements=%#v", request.PreparationRequirementsByActivity)
	}
}

func TestResolveBuildPreparationsReportsInvalidAssignmentsAndReferences(t *testing.T) {
	h := &Handler{data: &dataCatalogRichStub{}, environments: &environmentCatalogRichStub{}}
	request := ports.ExecutionRequest{Workflow: domain.WorkflowVersion{Activities: []domain.Activity{
		{ID: "activity", Command: domain.ActivityCommand{Executable: &domain.ExecutableReference{Source: domain.ExecutableSource{Type: domain.ExecutableSourceCatalog}}}},
	}}}
	if err := h.resolveBuildPreparations(context.Background(), &request); err == nil || !strings.Contains(err.Error(), "no plan assignment") {
		t.Fatalf("assignment error=%v", err)
	}
	request.Plan.Assignments = []domain.PlanAssignment{{ActivityID: "activity", ResourceID: "missing"}}
	if err := h.resolveBuildPreparations(context.Background(), &request); err == nil || !strings.Contains(err.Error(), "unknown resource") {
		t.Fatalf("resource error=%v", err)
	}
	request.Resources = []domain.Resource{{ID: "missing"}}
	if err := h.resolveBuildPreparations(context.Background(), &request); err == nil || !strings.Contains(err.Error(), "no artifact reference") {
		t.Fatalf("reference error=%v", err)
	}
}

func TestExecutionDetailIncludesDataPlaneObservations(t *testing.T) {
	run := domain.ExecutionRun{ID: "run"}
	data := &dataCatalogRichStub{instances: []domain.DataObjectInstance{{ID: "object"}}, dataLocations: []domain.DataLocation{{ID: "data-location"}}, materializations: []domain.ArtifactMaterialization{{ID: "materialization"}}, transferRuns: []domain.DataTransferRun{{ID: "transfer-run"}}}
	h := &Handler{executions: executionQueryStub{run: &run, tasks: []domain.TaskExecution{{ID: "task"}}, transfers: []domain.DataTransfer{{ID: "transfer"}}, handles: []domain.ActivityHandle{{ID: "handle"}}}, data: data}
	response := callHandler(t, http.MethodGet, "/", "", map[string]string{"runId": "run"}, h.GetExecution)
	if response.Code != 200 {
		t.Fatalf("detail=%d %q", response.Code, response.Body.String())
	}
	for _, value := range []string{"object", "data-location", "materialization", "transfer-run"} {
		if !strings.Contains(response.Body.String(), value) {
			t.Fatalf("detail lacks %s: %s", value, response.Body.String())
		}
	}
}

func TestMultipartBuildContextUploadAndUnavailableServices(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("context", "context.tar")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("archive"))
	_ = writer.Close()
	h := &Handler{build: buildOrchestratorStub{}}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	h.SaveBuildContext(response, request)
	if response.Code != 201 || !strings.Contains(response.Body.String(), "sha256:upload") {
		t.Fatalf("multipart=%d %q", response.Code, response.Body.String())
	}
	empty := &Handler{}
	for _, handler := range []http.HandlerFunc{empty.GenerateSSHKey, empty.ImportSSHKey, empty.ListSSHKeys, empty.SaveKubernetesToken, empty.FactoryReset, empty.TestEnvironmentConnection, empty.StreamBuildOutput, empty.StartArtifactBuildRun} {
		response := callHandler(t, http.MethodPost, "/", `{}`, nil, handler)
		if response.Code != 503 {
			t.Fatalf("unavailable %T=%d", handler, response.Code)
		}
	}
}

func TestCommonWritersAndValidationErrorBranches(t *testing.T) {
	response := httptest.NewRecorder()
	writeList(response, nil, fmt.Errorf("list failed"))
	if response.Code != 500 {
		t.Fatalf("writeList=%d", response.Code)
	}
	response = httptest.NewRecorder()
	var missing *domain.Resource
	writeItem(response, missing, nil)
	if response.Code != 404 {
		t.Fatalf("writeItem nil=%d", response.Code)
	}
	response = httptest.NewRecorder()
	writeItem(response, nil, fmt.Errorf("find failed"))
	if response.Code != 500 {
		t.Fatalf("writeItem error=%d", response.Code)
	}
	if !isNil(nil) || !isNil(missing) || isNil(domain.Resource{}) {
		t.Fatal("isNil mismatch")
	}
	instance := &instanceStoreRichStub{}
	h := &Handler{instance: instance}
	if result := callHandler(t, http.MethodPut, "/", `{"id":"","name":""}`, nil, h.SaveInstance); result.Code != 422 {
		t.Fatalf("invalid instance=%d", result.Code)
	}
	if result := callHandler(t, http.MethodPut, "/", `{"theme":"blue"}`, map[string]string{"clientId": "short"}, h.SaveUserPreferences); result.Code != 422 {
		t.Fatalf("invalid preferences=%d", result.Code)
	}
	environments := &environmentCatalogRichStub{}
	h.environments = environments
	if result := callHandler(t, http.MethodPut, "/", `{"environment":{"id":"other"}}`, map[string]string{"environmentId": "expected"}, h.ReplaceEnvironment); result.Code != 422 {
		t.Fatalf("replace mismatch=%d", result.Code)
	}
}

type executionErrorStub struct {
	ExecutionQuery
	run *domain.ExecutionRun
	err error
}

func (s executionErrorStub) FindRun(context.Context, string) (*domain.ExecutionRun, error) {
	return s.run, s.err
}

func TestRemainingCatalogHappyPathsAndErrors(t *testing.T) {
	topology := domain.NetworkTopology{ID: "topology"}
	topologies := &topologyStoreStub{topology: &topology}
	h := &Handler{topologies: topologies}
	if response := callHandler(t, http.MethodGet, "/", "", nil, h.ListNetworkTopologies); response.Code != 200 || !strings.Contains(response.Body.String(), "topology") {
		t.Fatalf("topologies=%d %q", response.Code, response.Body.String())
	}
	store := &workflowRepositoryStub{}
	h.workflows = store
	payload, err := apirequests.FromDomain(workflowFixture()).YAML()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/yaml")
	response := httptest.NewRecorder()
	h.CreateWorkflow(response, request)
	if response.Code != 201 || store.created == nil {
		t.Fatalf("create workflow=%d %q", response.Code, response.Body.String())
	}
	plans := planRepositoryStub{}
	h.plans = plans
	h.validator = validatorStub{}
	response = callHandler(t, http.MethodPost, "/", `{"plan":{"id":"plan"},"workflow":{},"resources":[],"executionScope":{},"networkTopology":{"id":"topology"}}`, nil, h.CreatePlan)
	if response.Code != 201 || !strings.Contains(response.Body.String(), "plan") {
		t.Fatalf("create plan=%d %q", response.Code, response.Body.String())
	}
}

func TestHandlerErrorBranchesForMutableCatalogOperations(t *testing.T) {
	operationErr := fmt.Errorf("operation failed")
	scope := &scopeStoreRichStub{err: sql.ErrNoRows}
	h := &Handler{scopes: scope}
	if response := callHandler(t, http.MethodDelete, "/", "", map[string]string{"scopeId": "missing"}, h.DeleteExecutionScope); response.Code != 404 {
		t.Fatalf("delete scope=%d", response.Code)
	}
	environments := &environmentCatalogRichStub{err: operationErr}
	h.environments = environments
	if response := callHandler(t, http.MethodPost, "/", `{"environment":{"id":"env"}}`, nil, h.CreateEnvironment); response.Code != 422 {
		t.Fatalf("create env=%d", response.Code)
	}
	if response := callHandler(t, http.MethodDelete, "/", "", map[string]string{"environmentId": "env"}, h.DeleteEnvironment); response.Code != 500 {
		t.Fatalf("delete env=%d", response.Code)
	}
	resources := &resourceInventoryRichStub{err: operationErr}
	h.resources = resources
	if response := callHandler(t, http.MethodPost, "/", `{"id":"resource"}`, nil, h.CreateResource); response.Code != 422 {
		t.Fatalf("create resource=%d", response.Code)
	}
	data := &dataCatalogRichStub{err: operationErr}
	h.data = data
	if response := callHandler(t, http.MethodPost, "/", `{"id":"materialization"}`, nil, h.SaveArtifactMaterialization); response.Code != 422 {
		t.Fatalf("save materialization=%d", response.Code)
	}
	if response := callHandler(t, http.MethodPost, "/", `{"id":"build"}`, nil, h.CreateArtifactBuild); response.Code != 422 {
		t.Fatalf("invalid build=%d", response.Code)
	}
	h.executions = executionErrorStub{err: operationErr}
	if response := callHandler(t, http.MethodGet, "/", "", map[string]string{"runId": "run"}, h.GetExecution); response.Code != 500 {
		t.Fatalf("execution error=%d", response.Code)
	}
	h.executions = executionErrorStub{}
	if response := callHandler(t, http.MethodGet, "/", "", map[string]string{"runId": "run"}, h.GetExecution); response.Code != 404 {
		t.Fatalf("execution missing=%d", response.Code)
	}
	monitor := connectionMonitorStub{err: operationErr}
	h.connections = monitor
	h.discovery = discoveryStub{err: operationErr}
	if response := callHandler(t, http.MethodPost, "/", "", map[string]string{"connectionId": "c"}, h.CheckEnvironmentConnection); response.Code != 422 {
		t.Fatalf("check=%d", response.Code)
	}
	if response := callHandler(t, http.MethodPost, "/", "", map[string]string{"connectionId": "c"}, h.DiscoverEnvironmentConnection); response.Code != 422 {
		t.Fatalf("discover=%d", response.Code)
	}
	if response := callHandler(t, http.MethodPut, "/", `{"id":"other","environmentId":"env"}`, map[string]string{"connectionId": "c"}, h.UpdateEnvironmentConnection); response.Code != 422 {
		t.Fatalf("update=%d", response.Code)
	}
}

func TestBuildCacheMissingContextAndResetFailures(t *testing.T) {
	build := &domain.ArtifactBuild{ID: "cached"}
	data := &dataCatalogRichStub{cachedBuild: build}
	h := &Handler{data: data}
	body := `{"id":"new","artifactVersionId":"version","contextDigest":"sha256:context","recipeDigest":"sha256:recipe","cacheKey":"cache"}`
	if response := callHandler(t, http.MethodPost, "/", body, nil, h.CreateArtifactBuild); response.Code != 200 || !strings.Contains(response.Body.String(), "cached") {
		t.Fatalf("cache=%d %q", response.Code, response.Body.String())
	}
	data.cachedBuild = nil
	if response := callHandler(t, http.MethodPost, "/", body, nil, h.CreateArtifactBuild); response.Code != 422 || !strings.Contains(response.Body.String(), "not uploaded") {
		t.Fatalf("context=%d %q", response.Code, response.Body.String())
	}
	h.build = buildOrchestratorStub{}
	if response := callHandler(t, http.MethodPost, "/", "", map[string]string{"buildId": "missing"}, h.StartArtifactBuildRun); response.Code != 404 {
		t.Fatalf("missing build=%d", response.Code)
	}
	h.factoryReset = func(context.Context) error { return fmt.Errorf("reset failed") }
	if response := callHandler(t, http.MethodPost, "/", "", nil, h.FactoryReset); response.Code != 422 {
		t.Fatalf("reset=%d", response.Code)
	}
	h.build = buildOrchestratorStub{err: fmt.Errorf("builder failed")}
	data.build = &domain.ArtifactBuild{ID: "build"}
	if response := callHandler(t, http.MethodGet, "/", "", map[string]string{"runId": "run"}, h.StreamBuildOutput); response.Code != 404 {
		t.Fatalf("output error=%d", response.Code)
	}
	if response := callHandler(t, http.MethodPost, "/", "", map[string]string{"buildId": "build"}, h.StartArtifactBuildRun); response.Code != 422 {
		t.Fatalf("start error=%d", response.Code)
	}
	if response := callHandler(t, http.MethodPost, "/", `{"artifactId":"tool","version":"1","image":"bad image"}`, nil, h.RegisterDockerArtifact); response.Code != 422 {
		t.Fatalf("docker validation=%d", response.Code)
	}
}
