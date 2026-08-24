package workflow_engine_api_handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"github.com/stretchr/testify/require"
)

type environmentRepositoryStub struct{ err error }

func (s environmentRepositoryStub) Create(context.Context, domain.EnvironmentDefinition) error {
	return s.err
}
func (s environmentRepositoryStub) Replace(context.Context, domain.EnvironmentDefinition) error {
	return s.err
}
func (s environmentRepositoryStub) Delete(context.Context, string) error { return s.err }
func (s environmentRepositoryStub) List(context.Context) ([]domain.EnvironmentDefinition, error) {
	return nil, s.err
}
func (s environmentRepositoryStub) Find(context.Context, string) (*domain.EnvironmentDefinition, error) {
	return nil, s.err
}
func (s environmentRepositoryStub) UpdateStatus(context.Context, string, domain.EnvironmentStatus) error {
	return s.err
}
func (s environmentRepositoryStub) UpsertConnection(context.Context, domain.EnvironmentConnection) error {
	return s.err
}
func (s environmentRepositoryStub) ListConnections(context.Context, string) ([]domain.EnvironmentConnection, error) {
	return nil, s.err
}

type workflowRepositoryStub struct {
	err        error
	definition *domain.WorkflowDefinition
	created    *domain.WorkflowDefinition
}

func (s *workflowRepositoryStub) Create(_ context.Context, value domain.WorkflowDefinition) error {
	s.created = &value
	return s.err
}
func (s *workflowRepositoryStub) List(context.Context) ([]domain.WorkflowDefinition, error) {
	if s.definition == nil {
		return nil, s.err
	}
	return []domain.WorkflowDefinition{*s.definition}, s.err
}
func (s *workflowRepositoryStub) Find(context.Context, string) (*domain.WorkflowDefinition, error) {
	return s.definition, s.err
}
func (s *workflowRepositoryStub) FindVersion(context.Context, string) (*domain.WorkflowVersion, error) {
	return nil, nil
}

type planRepositoryStub struct {
	plan *domain.SchedulePlan
	err  error
}

func (s planRepositoryStub) Save(context.Context, domain.SchedulePlan) error { return s.err }
func (s planRepositoryStub) List(context.Context) ([]domain.SchedulePlan, error) {
	return nil, s.err
}
func (s planRepositoryStub) Find(context.Context, string) (*domain.SchedulePlan, error) {
	return s.plan, s.err
}

type eventPublisherStub struct {
	jobs []domainqueue.Job
	err  error
}

func (s *eventPublisherStub) Publish(_ context.Context, job domainqueue.Job) (domainqueue.Job, error) {
	s.jobs = append(s.jobs, job)
	return job, s.err
}

type validatorStub struct{ err error }

func (s validatorStub) Validate(domain.SchedulePlan, domain.WorkflowVersion, []domain.Resource, domain.ExecutionScope, domain.NetworkTopology) error {
	return s.err
}

type executionQueryStub struct {
	run       *domain.ExecutionRun
	tasks     []domain.TaskExecution
	transfers []domain.DataTransfer
	handles   []domain.ActivityHandle
	events    []domainevents.Event
}

type topologyStoreStub struct {
	topology *domain.NetworkTopology
	created  *domain.NetworkTopology
	err      error
}

func (s *topologyStoreStub) Create(_ context.Context, topology domain.NetworkTopology) error {
	s.created = &topology
	return s.err
}
func (s *topologyStoreStub) Find(context.Context, string) (*domain.NetworkTopology, error) {
	return s.topology, s.err
}
func (s *topologyStoreStub) List(context.Context) ([]domain.NetworkTopology, error) {
	if s.topology == nil {
		return nil, s.err
	}
	return []domain.NetworkTopology{*s.topology}, s.err
}
func (s *topologyStoreStub) CreateScope(context.Context, domain.ExecutionScope) error { return s.err }
func (s *topologyStoreStub) DeleteScope(context.Context, string) error                { return s.err }
func (s *topologyStoreStub) FindScope(context.Context, string) (*domain.ExecutionScope, error) {
	return nil, s.err
}
func (s *topologyStoreStub) ListScopes(context.Context) ([]domain.ExecutionScope, error) {
	return nil, s.err
}

func (s executionQueryStub) FindRun(context.Context, string) (*domain.ExecutionRun, error) {
	return s.run, nil
}
func (s executionQueryStub) ListRuns(context.Context) ([]domain.ExecutionRun, error) {
	if s.run == nil {
		return nil, nil
	}
	return []domain.ExecutionRun{*s.run}, nil
}
func (s executionQueryStub) ListRunsPage(_ context.Context, page, pageSize int, _, _, _ string) (domain.ExecutionRunPage, error) {
	items, _ := s.ListRuns(context.Background())
	return domain.ExecutionRunPage{Items: items, Page: page, PageSize: pageSize, Total: len(items)}, nil
}
func (s executionQueryStub) ListTasks(context.Context, string) ([]domain.TaskExecution, error) {
	return s.tasks, nil
}
func (s executionQueryStub) ListTransfers(context.Context, string) ([]domain.DataTransfer, error) {
	return s.transfers, nil
}
func (s executionQueryStub) ListHandles(context.Context, string) ([]domain.ActivityHandle, error) {
	return s.handles, nil
}
func (s executionQueryStub) ListEvents(context.Context, string) ([]domainevents.Event, error) {
	return s.events, nil
}

type resourceInventoryStub struct{}

type instanceStoreStub struct {
	value *domaininstance.Instance
}

func (store *instanceStoreStub) Find(context.Context) (*domaininstance.Instance, error) {
	return store.value, nil
}

func (store *instanceStoreStub) Save(_ context.Context, value domaininstance.Instance) error {
	store.value = &value
	return nil
}

func (store *instanceStoreStub) FindPreferences(context.Context, string) (*domaininstance.UserPreferences, error) {
	return nil, nil
}

func (store *instanceStoreStub) SavePreferences(context.Context, domaininstance.UserPreferences) error {
	return nil
}

func (resourceInventoryStub) Upsert(context.Context, domain.Resource) error { return nil }
func (resourceInventoryStub) UpsertRuntimeBinding(context.Context, domain.ResourceRuntimeBinding) error {
	return nil
}
func (resourceInventoryStub) UpsertRelation(context.Context, domain.ResourceRelation) error {
	return nil
}
func (resourceInventoryStub) List(context.Context) ([]domain.Resource, error) { return nil, nil }
func (resourceInventoryStub) FindByID(context.Context, string) (*domain.Resource, error) {
	return nil, nil
}
func (resourceInventoryStub) FindByProviderID(context.Context, string, string) (*domain.Resource, error) {
	return nil, nil
}
func (resourceInventoryStub) ListByRuntime(context.Context, string, string) ([]domain.Resource, error) {
	return nil, nil
}
func (resourceInventoryStub) ListSchedulable(context.Context, string) ([]domain.Resource, error) {
	return nil, nil
}
func (resourceInventoryStub) CreateSnapshot(context.Context, domain.ResourceSnapshot) error {
	return nil
}
func (resourceInventoryStub) LatestSnapshot(context.Context, string) (*domain.ResourceSnapshot, error) {
	return nil, nil
}

func newTestHandler() *Handler {
	handler, err := New(Dependencies{
		Environments: environmentRepositoryStub{}, Workflows: &workflowRepositoryStub{},
		Plans: planRepositoryStub{}, Events: &eventPublisherStub{}, Validator: validatorStub{},
		Executions: executionQueryStub{},
		Topologies: &topologyStoreStub{},
		Scopes:     &topologyStoreStub{},
		Resources:  resourceInventoryStub{},
		Instance:   &instanceStoreStub{},
	})
	if err != nil {
		panic(err)
	}
	return handler
}

func workflowFixture() domain.WorkflowDefinition {
	return domain.WorkflowDefinition{
		ID: "source", ExternalID: "source", Name: "Source", Namespace: "science",
		Types: []domain.ActivityType{{ID: "source-activity", Name: "Source activity"}},
		Version: domain.WorkflowVersion{
			ID: "source-v1", WorkflowID: "source", Version: 1,
			Activities: []domain.Activity{{
				ID: "prepare", Name: "Prepare", WorkflowVersionID: "source-v1", ActivityTypeID: "source-activity",
				Command: domain.ActivityCommand{
					Entrypoint: "sh", Arguments: []string{"-c", "echo ready"},
					Executable: &domain.ExecutableReference{
						Source:   domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "alpine:3.20"},
						Delivery: domain.ExecutableDelivery{Strategy: domain.DeliveryManaged},
					},
				},
			}},
		},
	}
}

func TestExportWorkflowReturnsPortableYAML(t *testing.T) {
	store := &workflowRepositoryStub{definition: ptr(workflowFixture())}
	handler := newTestHandler()
	handler.workflows = store
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/workflow-definitions/source/export/", nil)
	request.SetPathValue("workflowId", "source")
	handler.ExportWorkflow(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/yaml")
	require.Contains(t, recorder.Body.String(), "name: Source")
	require.Contains(t, recorder.Body.String(), "activities:")
}

func TestDuplicateWorkflowCreatesIndependentDefinition(t *testing.T) {
	store := &workflowRepositoryStub{definition: ptr(workflowFixture())}
	handler := newTestHandler()
	handler.workflows = store
	recorder := httptest.NewRecorder()
	request := yamlRequest("name: Copied workflow\nnamespace: copied\n")
	request.SetPathValue("workflowId", "source")
	handler.DuplicateWorkflow(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotNil(t, store.created)
	require.Equal(t, "copied-workflow", store.created.ID)
	require.Equal(t, "copied", store.created.Namespace)
	require.Equal(t, "copied-workflow-v1", store.created.Version.ID)
	require.Equal(t, "copied-workflow-activity", store.created.Version.Activities[0].ActivityTypeID)
}

func TestDuplicateWorkflowRejectsMissingNameAndUnknownWorkflow(t *testing.T) {
	handler := newTestHandler()
	recorder := httptest.NewRecorder()
	request := yamlRequest("{}")
	request.SetPathValue("workflowId", "source")
	handler.DuplicateWorkflow(recorder, request)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	recorder = httptest.NewRecorder()
	request = yamlRequest("name: Copy")
	request.SetPathValue("workflowId", "missing")
	handler.DuplicateWorkflow(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func ptr[T any](value T) *T { return &value }

func TestNewRejectsIncompleteDependencies(t *testing.T) {
	if _, err := New(Dependencies{}); err == nil {
		t.Fatal("missing dependencies must fail")
	}
}

func TestCreateEnvironment(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := yamlRequest("environment:\n  id: env\nversion:\n  id: env-1\n")
	newTestHandler().CreateEnvironment(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":"env"`)
}

func TestCreateEnvironmentAcceptsYAML(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`
environment:
  id: env-yaml
  name: YAML environment
  status: ready
version:
  id: env-yaml-v1
  environmentId: env-yaml
  version: 1
`))
	request.Header.Set("Content-Type", "application/yaml")
	newTestHandler().CreateEnvironment(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":"env-yaml"`)
}

func TestCreateAndGetNetworkTopology(t *testing.T) {
	store := &topologyStoreStub{}
	handler := newTestHandler()
	handler.topologies = store
	recorder := httptest.NewRecorder()
	request := yamlRequest(`
id: federated-v1
name: Federated network
version: 1
executionScopeId: hybrid
links:
  - id: hpc-cloud
    sourceResourceId: hpc-node
    targetResourceId: cloud-vm
    bandwidthBitsPerSecond: 500000000
    latencySeconds: 0.1
    bidirectional: true
`)
	handler.CreateNetworkTopology(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "federated-v1", store.created.ID)
	require.Equal(t, 0.1, store.created.Links[0].LatencySeconds)

	store.topology = store.created
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/network-topologies/federated-v1", nil)
	request.SetPathValue("topologyId", "federated-v1")
	handler.GetNetworkTopology(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"executionScopeId":"hybrid"`)
}

func TestGetNetworkTopologyReturnsNotFound(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/network-topologies/missing", nil)
	request.SetPathValue("topologyId", "missing")
	newTestHandler().GetNetworkTopology(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestYAMLRejectsUnknownField(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("unknown: true\n"))
	request.Header.Set("Content-Type", "application/yaml; charset=utf-8")
	newTestHandler().CreateWorkflow(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCreateWorkflowRejectsUnknownYAMLField(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := yamlRequest("unknown: true\n")
	newTestHandler().CreateWorkflow(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCreatePlanReturnsValidationError(t *testing.T) {
	handler := newTestHandler()
	handler.validator = validatorStub{err: errors.New("invalid plan")}
	recorder := httptest.NewRecorder()
	request := yamlRequest("plan: {}\nworkflow: {}\nresources: []\n")
	handler.CreatePlan(recorder, request)
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid plan")
}

func TestGetPlanReturnsPlanAndNotFound(t *testing.T) {
	plan := &domain.SchedulePlan{ID: "plan"}
	handler := newTestHandler()
	handler.plans = planRepositoryStub{plan: plan}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/plans/plan", nil)
	request.SetPathValue("planId", "plan")
	handler.GetPlan(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	handler.plans = planRepositoryStub{}
	recorder = httptest.NewRecorder()
	handler.GetPlan(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestGetExecutionReturnsActivitiesAndEvents(t *testing.T) {
	handler := newTestHandler()
	handler.executions = executionQueryStub{
		run:   &domain.ExecutionRun{ID: "run", Status: domain.ExecutionRunRunning},
		tasks: []domain.TaskExecution{{ID: "run:activity", ActivityID: "activity", Status: domain.TaskRunning}},
		transfers: []domain.DataTransfer{{ID: "transfer", ExecutionRunID: "run",
			ProducerActivityID: "producer", ConsumerActivityID: "activity", Bytes: 1024}},
		handles: []domain.ActivityHandle{{ID: "run:activity", Artifacts: &domain.ArtifactManifest{
			SchemaVersion: 1,
		}}},
		events: []domainevents.Event{{ID: "event", Type: domainevents.ActivityStarted,
			AggregateType: "activity_execution", AggregateID: "run:activity"}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/executions/run", nil)
	request.SetPathValue("runId", "run")
	handler.GetExecution(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"eventType":"activity.started"`)
	require.Contains(t, recorder.Body.String(), `"schemaVersion":1`)
	require.Contains(t, recorder.Body.String(), `"dataTransfers":[{"id":"transfer"`)
}

func TestCreateExecutionPublishesPersistentCommand(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := yamlRequest("run:\n  id: run\n  mode: simulation\nplan:\n  id: plan\nworkflow:\n  id: workflow\nresources: []\nnetworkTopology: {}\nactivityProfiles: []\n")
	newTestHandler().CreateExecution(recorder, request)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"eventType":"execution.run.requested"`)
}

func TestJSONRequestsAreAccepted(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"environment":{"id":"env"}}`))
	request.Header.Set("Content-Type", "application/json")
	newTestHandler().CreateEnvironment(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code)
}

func yamlRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/yaml")
	return request
}
