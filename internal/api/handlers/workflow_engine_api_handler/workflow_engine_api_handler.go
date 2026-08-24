package workflow_engine_api_handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	applicationbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/sshkey"
	"gopkg.in/yaml.v3"
)

const maxRequestBodyBytes = 32 << 20

type ExecutionQuery interface {
	FindRun(context.Context, string) (*domain.ExecutionRun, error)
	ListRuns(context.Context) ([]domain.ExecutionRun, error)
	ListRunsPage(context.Context, int, int, string, string, string) (domain.ExecutionRunPage, error)
	ListTasks(context.Context, string) ([]domain.TaskExecution, error)
	ListTransfers(context.Context, string) ([]domain.DataTransfer, error)
	ListHandles(context.Context, string) ([]domain.ActivityHandle, error)
	ListEvents(context.Context, string) ([]domainevents.Event, error)
}
type StorageNavigator interface {
	List(context.Context, string) ([]domain.StorageResource, error)
	Roots(context.Context, string) ([]domain.StorageBrowseRoot, error)
	Browse(context.Context, string, domain.BrowseRequest) (domain.BrowsePage, error)
	Stat(context.Context, string, string) (domain.FileEntry, error)
	StartDownload(context.Context, string, string, string) (domain.DownloadRun, error)
	OpenDownload(context.Context, string) (io.ReadCloser, domain.FileEntry, error)
	Download(context.Context, string) (*domain.DownloadRun, error)
	Checksum(context.Context, string, string) (string, error)
	QueueCopy(context.Context, string, string, string, string) (domain.DownloadRun, error)
	QueueArchive(context.Context, string, string, string) (domain.DownloadRun, error)
	PromoteData(context.Context, string, string, string, string, string, string) error
	PromoteArtifact(context.Context, string, string, string, string, string, string, string) error
	IndexRuns(context.Context, string) ([]domain.IndexRun, error)
	StartIndex(context.Context, string, string) (domain.IndexRun, error)
	Delete(context.Context, string, string) error
}
type BuildOrchestrator interface {
	Upload(context.Context, io.Reader) (domain.BuildContextArtifact, error)
	Start(context.Context, domain.ArtifactBuild) (domain.BuildRun, error)
	OpenOutput(context.Context, string) (io.ReadCloser, string, error)
	MaxUploadBytes() int64
}

type DockerArtifactRequest struct {
	ArtifactID   string `json:"artifactId"`
	Version      string `json:"version"`
	Image        string `json:"image"`
	Architecture string `json:"architecture"`
}

type Dependencies struct {
	Environments ports.EnvironmentCatalog
	Workflows    ports.WorkflowStore
	Plans        ports.PlanStore
	Events       ports.EventPublisher
	Validator    ports.PlanValidator
	Executions   ExecutionQuery
	Topologies   ports.NetworkTopologyStore
	Scopes       ports.ExecutionScopeStore
	Data         ports.DataCatalog
	Resources    ports.ResourceInventory
	Instance     ports.InstanceStore
	Connections  ports.ConnectionHealthMonitor
	Discovery    ports.EnvironmentDiscovery
	SSHKeys      *sshkey.Manager
	Audit        ports.AuditStore
	Console      ports.ConsoleCommands
	Terminal     ports.InteractiveConsole
	Storage      StorageNavigator
	Build        BuildOrchestrator
}

type Handler struct {
	environments ports.EnvironmentCatalog
	workflows    ports.WorkflowStore
	plans        ports.PlanStore
	events       ports.EventPublisher
	validator    ports.PlanValidator
	executions   ExecutionQuery
	topologies   ports.NetworkTopologyStore
	scopes       ports.ExecutionScopeStore
	data         ports.DataCatalog
	resources    ports.ResourceInventory
	instance     ports.InstanceStore
	connections  ports.ConnectionHealthMonitor
	discovery    ports.EnvironmentDiscovery
	sshKeys      *sshkey.Manager
	audit        ports.AuditStore
	console      ports.ConsoleCommands
	terminal     ports.InteractiveConsole
	storage      StorageNavigator
	build        BuildOrchestrator
}

// SearchResult is a compact, navigable projection of a control-plane entity.
// It intentionally does not expose raw configuration or credentials.
type SearchResult struct {
	Type     string  `json:"type"`
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Subtitle string  `json:"subtitle,omitempty"`
	Path     string  `json:"path"`
	Status   string  `json:"status,omitempty"`
	Score    float64 `json:"score"`
}

func New(dependencies Dependencies) (*Handler, error) {
	if missingDependencies(dependencies) {
		return nil, fmt.Errorf("workflow API dependencies are required")
	}
	return &Handler{
		environments: dependencies.Environments, workflows: dependencies.Workflows,
		plans: dependencies.Plans, events: dependencies.Events,
		validator: dependencies.Validator, executions: dependencies.Executions,
		topologies:  dependencies.Topologies,
		scopes:      dependencies.Scopes,
		data:        dependencies.Data,
		resources:   dependencies.Resources,
		instance:    dependencies.Instance,
		connections: dependencies.Connections,
		discovery:   dependencies.Discovery,
		sshKeys:     dependencies.SSHKeys,
		audit:       dependencies.Audit,
		console:     dependencies.Console,
		terminal:    dependencies.Terminal,
		storage:     dependencies.Storage,
		build:       dependencies.Build,
	}, nil
}

func (h *Handler) ListStorages(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	items, e := h.storage.List(r.Context(), r.PathValue("environmentId"))
	writeList(w, items, e)
}
func (h *Handler) StorageRoots(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	items, e := h.storage.Roots(r.Context(), r.PathValue("storageId"))
	writeList(w, items, e)
}
func (h *Handler) BrowseStorage(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	v, e := h.storage.Browse(r.Context(), r.PathValue("storageId"), domain.BrowseRequest{Path: r.URL.Query().Get("path"), Cursor: r.URL.Query().Get("cursor"), Limit: limit})
	if e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) StatStorageEntry(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	v, e := h.storage.Stat(r.Context(), r.PathValue("storageId"), r.URL.Query().Get("path"))
	if e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) CreateDownload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	var v struct {
		Path string `json:"path"`
		ID   string `json:"id"`
	}
	if !decode(w, r, &v) {
		return
	}
	if v.ID == "" {
		v.ID = fmt.Sprintf("download-%d", time.Now().UnixNano())
	}
	out, e := h.storage.StartDownload(r.Context(), r.PathValue("storageId"), v.Path, v.ID)
	if e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
func (h *Handler) StreamDownload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	body, entry, e := h.storage.OpenDownload(r.Context(), r.PathValue("downloadId"))
	if e != nil {
		writeError(w, http.StatusNotFound, e)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", entry.Name))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, body)
}
func (h *Handler) GetDownload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	v, e := h.storage.Download(r.Context(), r.PathValue("downloadId"))
	if e != nil {
		writeError(w, http.StatusNotFound, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) ChecksumStorageEntry(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	var in struct {
		Path string `json:"path"`
	}
	if !decode(w, r, &in) {
		return
	}
	sum, e := h.storage.Checksum(r.Context(), r.PathValue("storageId"), in.Path)
	if e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"checksum": sum})
}
func (h *Handler) DeleteStorageEntry(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("storage browser is unavailable"))
		return
	}
	if e := h.storage.Delete(r.Context(), r.PathValue("storageId"), r.URL.Query().Get("path")); e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) CopyStorageEntry(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path                 string `json:"path"`
		DestinationStorageID string `json:"destinationStorageId"`
		ID                   string `json:"id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ID == "" {
		in.ID = fmt.Sprintf("copy-%d", time.Now().UnixNano())
	}
	v, e := h.storage.QueueCopy(r.Context(), r.PathValue("storageId"), in.Path, in.DestinationStorageID, in.ID)
	if e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}
func (h *Handler) ArchiveStorageDirectory(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path string `json:"path"`
		ID   string `json:"id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ID == "" {
		in.ID = fmt.Sprintf("archive-%d", time.Now().UnixNano())
	}
	v, e := h.storage.QueueArchive(r.Context(), r.PathValue("storageId"), in.Path, in.ID)
	if e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}
func (h *Handler) PromoteStorageData(w http.ResponseWriter, r *http.Request) {
	var in struct{ Path, WorkflowVersionID, RunID, ActivityID, ID string }
	if !decode(w, r, &in) {
		return
	}
	if in.ID == "" {
		in.ID = fmt.Sprintf("data-%d", time.Now().UnixNano())
	}
	if e := h.storage.PromoteData(r.Context(), r.PathValue("storageId"), in.Path, in.WorkflowVersionID, in.RunID, in.ActivityID, in.ID); e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": in.ID})
}
func (h *Handler) PromoteStorageArtifact(w http.ResponseWriter, r *http.Request) {
	var in struct{ Path, ID, Name, Version, Scope, ScopeID string }
	if !decode(w, r, &in) {
		return
	}
	if in.ID == "" {
		in.ID = fmt.Sprintf("artifact-%d", time.Now().UnixNano())
	}
	if e := h.storage.PromoteArtifact(r.Context(), r.PathValue("storageId"), in.Path, in.ID, in.Name, in.Version, in.Scope, in.ScopeID); e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": in.ID})
}
func (h *Handler) ListStorageIndexRuns(w http.ResponseWriter, r *http.Request) {
	v, e := h.storage.IndexRuns(r.Context(), r.PathValue("storageId"))
	writeList(w, v, e)
}
func (h *Handler) StartStorageIndex(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID string `json:"id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ID == "" {
		in.ID = fmt.Sprintf("index-%d", time.Now().UnixNano())
	}
	v, e := h.storage.StartIndex(r.Context(), r.PathValue("storageId"), in.ID)
	if e != nil {
		writeError(w, http.StatusUnprocessableEntity, e)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}

func (h *Handler) OpenConsoleSession(w http.ResponseWriter, r *http.Request) {
	if h.terminal == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("interactive console is unavailable"))
		return
	}
	var request domainconsole.SessionRequest
	if !decode(w, r, &request) {
		return
	}
	session, err := h.terminal.OpenSession(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) StreamConsoleSession(w http.ResponseWriter, r *http.Request) {
	if h.terminal == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("interactive console is unavailable"))
		return
	}
	h.terminal.StreamSession(w, r, r.PathValue("sessionId"))
}

func (h *Handler) ListConsoleSessions(w http.ResponseWriter, r *http.Request) {
	if h.terminal == nil {
		writeJSON(w, http.StatusOK, []domainconsole.Session{})
		return
	}
	items, err := h.terminal.ListSessions(r.Context())
	writeList(w, items, err)
}

func (h *Handler) CloseConsoleSession(w http.ResponseWriter, r *http.Request) {
	if h.terminal == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("interactive console is unavailable"))
		return
	}
	if err := h.terminal.CloseSession(r.Context(), r.PathValue("sessionId")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ExportConsoleSessionLog(w http.ResponseWriter, r *http.Request) {
	if h.terminal == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("interactive console is unavailable"))
		return
	}
	log, err := h.terminal.SessionLog(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=akoflow-"+r.PathValue("sessionId")+".log")
	_, _ = w.Write(log)
}

func (h *Handler) ExecuteConsoleCommand(w http.ResponseWriter, r *http.Request) {
	if h.console == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("console is unavailable"))
		return
	}
	var request domainconsole.Request
	if !decode(w, r, &request) {
		return
	}
	command, err := h.console.ExecuteCommand(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, command)
}

func (h *Handler) ListConsoleCommands(w http.ResponseWriter, r *http.Request) {
	if h.console == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("console is unavailable"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.console.ListCommands(r.Context(), limit)
	writeList(w, items, err)
}

func (h *Handler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if h.audit == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("audit is unavailable"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := domainaudit.Filter{EventType: r.URL.Query().Get("eventType"), EnvironmentID: r.URL.Query().Get("environmentId"),
		ResourceID: r.URL.Query().Get("resourceId"), ConnectionID: r.URL.Query().Get("connectionId"),
		SessionID: r.URL.Query().Get("sessionId"), ExecutionID: r.URL.Query().Get("executionId"),
		Outcome: domainaudit.Outcome(r.URL.Query().Get("outcome")), Limit: limit}
	items, err := h.audit.ListAuditEvents(r.Context(), filter)
	writeList(w, items, err)
}

func (h *Handler) GenerateSSHKey(w http.ResponseWriter, r *http.Request) {
	if h.sshKeys == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SSH key management is unavailable"))
		return
	}
	var request struct {
		ID      string `json:"id"`
		Comment string `json:"comment"`
	}
	if !decode(w, r, &request) {
		return
	}
	key, err := h.sshKeys.Generate(request.ID, request.Comment)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

func (h *Handler) ListSSHKeys(w http.ResponseWriter, r *http.Request) {
	if h.sshKeys == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("SSH key management is unavailable"))
		return
	}
	keys, err := h.sshKeys.List()
	writeList(w, keys, err)
}

func (h *Handler) DiscoverEnvironmentConnection(w http.ResponseWriter, r *http.Request) {
	if h.discovery == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("environment discovery is unavailable"))
		return
	}
	snapshots, err := h.discovery.DiscoverConnection(r.Context(), r.PathValue("connectionId"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snapshots})
}

func (h *Handler) CheckEnvironmentConnection(w http.ResponseWriter, r *http.Request) {
	if h.connections == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("connection monitoring is unavailable"))
		return
	}
	check, err := h.connections.Check(r.Context(), r.PathValue("connectionId"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, check)
}

func (h *Handler) UpdateEnvironmentConnection(w http.ResponseWriter, r *http.Request) {
	var connection domain.EnvironmentConnection
	if !decode(w, r, &connection) {
		return
	}
	if connection.ID == "" || connection.ID != r.PathValue("connectionId") || connection.EnvironmentID == "" {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("connection id and environment id are required"))
		return
	}
	if err := h.environments.UpsertConnection(r.Context(), connection); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, connection)
}

func (h *Handler) ListEnvironmentConnectionHistory(w http.ResponseWriter, r *http.Request) {
	if h.connections == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("connection monitoring is unavailable"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	checks, err := h.connections.History(r.Context(), r.PathValue("connectionId"), limit)
	writeList(w, checks, err)
}

func missingDependencies(dependencies Dependencies) bool {
	return dependencies.Environments == nil || dependencies.Workflows == nil ||
		dependencies.Plans == nil || dependencies.Events == nil ||
		dependencies.Validator == nil || dependencies.Executions == nil ||
		dependencies.Topologies == nil || dependencies.Scopes == nil || dependencies.Resources == nil ||
		dependencies.Instance == nil
}

func (h *Handler) CreateExecutionScope(w http.ResponseWriter, r *http.Request) {
	var scope domain.ExecutionScope
	if !decode(w, r, &scope) {
		return
	}
	if err := h.scopes.CreateScope(r.Context(), scope); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, scope)
}

func (h *Handler) DeleteExecutionScope(w http.ResponseWriter, r *http.Request) {
	if err := h.scopes.DeleteScope(r.Context(), r.PathValue("scopeId")); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, fmt.Errorf("execution scope not found"))
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListExecutionScopes(w http.ResponseWriter, r *http.Request) {
	items, err := h.scopes.ListScopes(r.Context())
	writeList(w, items, err)
}

func (h *Handler) GetExecutionScope(w http.ResponseWriter, r *http.Request) {
	item, err := h.scopes.FindScope(r.Context(), r.PathValue("scopeId"))
	writeItem(w, item, err)
}

func (h *Handler) GetInstance(w http.ResponseWriter, r *http.Request) {
	value, err := h.instance.Find(r.Context())
	writeItem(w, value, err)
}

func (h *Handler) SaveInstance(w http.ResponseWriter, r *http.Request) {
	var value domaininstance.Instance
	if !decode(w, r, &value) {
		return
	}
	if strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.Name) == "" {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("instance id and name are required"))
		return
	}
	if err := h.instance.Save(r.Context(), value); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	stored, err := h.instance.Find(r.Context())
	writeItem(w, stored, err)
}

func (h *Handler) GetUserPreferences(w http.ResponseWriter, r *http.Request) {
	value, err := h.instance.FindPreferences(r.Context(), r.PathValue("clientId"))
	writeItem(w, value, err)
}

func (h *Handler) SaveUserPreferences(w http.ResponseWriter, r *http.Request) {
	var value domaininstance.UserPreferences
	if !decode(w, r, &value) {
		return
	}
	value.ClientID = r.PathValue("clientId")
	if len(value.ClientID) < 8 || len(value.ClientID) > 128 {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("a valid client id is required"))
		return
	}
	if value.Theme != "light" && value.Theme != "dark" {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("theme must be light or dark"))
		return
	}
	if err := h.instance.SavePreferences(r.Context(), value); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	stored, err := h.instance.FindPreferences(r.Context(), value.ClientID)
	writeItem(w, stored, err)
}

func (h *Handler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	items, err := h.environments.List(r.Context())
	writeList(w, items, err)
}

func (h *Handler) GetEnvironment(w http.ResponseWriter, r *http.Request) {
	item, err := h.environments.Find(r.Context(), r.PathValue("environmentId"))
	writeItem(w, item, err)
}

func (h *Handler) ListResources(w http.ResponseWriter, r *http.Request) {
	items, err := h.resources.List(r.Context())
	writeList(w, items, err)
}

func (h *Handler) GetResource(w http.ResponseWriter, r *http.Request) {
	item, err := h.resources.FindByID(r.Context(), r.PathValue("resourceId"))
	writeItem(w, item, err)
}

func (h *Handler) GetResourceSnapshot(w http.ResponseWriter, r *http.Request) {
	item, err := h.resources.LatestSnapshot(r.Context(), r.PathValue("resourceId"))
	writeItem(w, item, err)
}

func (h *Handler) CreateResource(w http.ResponseWriter, r *http.Request) {
	var item domain.Resource
	if !decode(w, r, &item) {
		return
	}
	if err := h.resources.Upsert(r.Context(), item); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) ListNetworkTopologies(w http.ResponseWriter, r *http.Request) {
	items, err := h.topologies.List(r.Context())
	writeList(w, items, err)
}

func (h *Handler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	items, err := h.workflows.List(r.Context())
	writeList(w, items, err)
}

func (h *Handler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	item, err := h.workflows.Find(r.Context(), r.PathValue("workflowId"))
	writeItem(w, item, err)
}

// ExportWorkflow returns the same portable YAML authoring contract accepted by
// workflow import. Generated IDs and resolved runtime paths are not exported.
func (h *Handler) ExportWorkflow(w http.ResponseWriter, r *http.Request) {
	definition, err := h.workflows.Find(r.Context(), r.PathValue("workflowId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if definition == nil {
		writeError(w, http.StatusNotFound, nil)
		return
	}
	payload, err := apirequests.FromDomain(*definition).YAML()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", definition.ID+".yaml"))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	items, err := h.plans.List(r.Context())
	writeList(w, items, err)
}

// Search is the global, permission-aware control-plane search. It uses the
// same catalogs as the individual pages, so no client-side bulk indexing is
// needed and every result has a direct UI route.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, map[string]any{"query": query, "results": []SearchResult{}, "total": 0})
		return
	}
	limit := positiveInteger(r.URL.Query().Get("limit"), 30)
	if limit > 100 {
		limit = 100
	}
	types := searchTypes(r.URL.Query().Get("types"))
	results := make([]SearchResult, 0, limit)
	add := func(item SearchResult, fields ...string) {
		if !types[item.Type] {
			return
		}
		if score := searchScore(query, fields...); score > 0 {
			item.Score = score
			results = append(results, item)
		}
	}
	if types["workflow"] {
		items, err := h.workflows.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			add(SearchResult{Type: "workflow", ID: item.ID, Title: item.Name, Subtitle: item.Namespace, Path: "/workflows/" + url.PathEscape(item.ID)}, item.ID, item.ExternalID, item.Name, item.Namespace)
		}
	}
	if types["execution"] {
		items, err := h.executions.ListRuns(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			add(SearchResult{Type: "execution", ID: item.ID, Title: firstNonEmpty(item.Title, item.ID), Subtitle: firstNonEmpty(item.FailureReason, item.ResourceID, item.RuntimeID), Status: string(item.Status), Path: "/executions/" + url.PathEscape(item.ID)}, item.ID, item.Title, item.ResourceID, item.RuntimeID, item.FailureReason, string(item.Status))
		}
	}
	if types["artifact"] {
		items, err := h.data.ListArtifacts(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			add(SearchResult{Type: "artifact", ID: item.ID, Title: firstNonEmpty(item.Name, item.ID), Subtitle: item.Version, Path: "/artifacts/" + url.PathEscape(item.ID)}, item.ID, item.Name, item.Version)
		}
	}
	if types["environment"] {
		items, err := h.environments.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			value := item.Environment
			add(SearchResult{Type: "environment", ID: value.ID, Title: value.Name, Subtitle: value.Description, Status: string(value.Status), Path: "/environments/" + url.PathEscape(value.ID)}, value.ID, value.Name, value.Description, string(value.Status))
		}
	}
	if types["resource"] {
		items, err := h.resources.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			add(SearchResult{Type: "resource", ID: item.ID, Title: item.Name, Subtitle: string(item.Type), Path: "/resources/" + url.PathEscape(item.ID)}, item.ID, item.Name, item.ProviderID, item.Region, item.Zone, string(item.Type), item.EnvironmentVersionID)
		}
	}
	if types["plan"] {
		items, err := h.plans.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			add(SearchResult{Type: "plan", ID: item.ID, Title: item.ID, Subtitle: item.Algorithm, Path: "/plans/" + url.PathEscape(item.ID)}, item.ID, item.Algorithm, item.Objective, item.WorkflowVersionID, item.ExecutionScopeID)
		}
	}
	if types["scope"] {
		items, err := h.scopes.ListScopes(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			add(SearchResult{Type: "scope", ID: item.ID, Title: item.Name, Subtitle: item.NetworkTopologyID, Path: "/execution-scopes/" + url.PathEscape(item.ID)}, item.ID, item.Name, item.NetworkTopologyID, strings.Join(item.EnvironmentVersionIDs, " "))
		}
	}
	if types["materialization"] {
		items, err := h.data.ListArtifactMaterializations(r.Context(), "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, item := range items {
			add(SearchResult{Type: "materialization", ID: item.ID, Title: item.VariantID, Subtitle: firstNonEmpty(item.DestinationPath, item.ResourceID), Status: string(item.Status), Path: "/materializations?runId=" + url.QueryEscape(item.RunID)}, item.ID, item.VariantID, item.Digest, item.ResourceID, item.RunID, item.ActivityID, item.DestinationPath, string(item.Status))
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"query": query, "results": results, "total": len(results)})
}

func searchTypes(raw string) map[string]bool {
	all := map[string]bool{"workflow": true, "execution": true, "artifact": true, "environment": true, "resource": true, "plan": true, "scope": true, "materialization": true}
	if strings.TrimSpace(raw) == "" {
		return all
	}
	selected := map[string]bool{}
	for _, value := range strings.Split(raw, ",") {
		if all[strings.TrimSpace(value)] {
			selected[strings.TrimSpace(value)] = true
		}
	}
	return selected
}

func searchScore(query string, fields ...string) float64 {
	needle := strings.ToLower(strings.TrimSpace(query))
	best := 0.0
	for _, field := range fields {
		value := strings.ToLower(field)
		switch {
		case value == needle:
			best = max(best, 1)
		case strings.HasPrefix(value, needle):
			best = max(best, .9)
		case strings.Contains(value, needle):
			best = max(best, .7)
		}
	}
	return best
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (h *Handler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("page") || r.URL.Query().Has("pageSize") {
		page := positiveInteger(r.URL.Query().Get("page"), 1)
		pageSize := positiveInteger(r.URL.Query().Get("pageSize"), 20)
		result, err := h.executions.ListRunsPage(r.Context(), page, pageSize,
			r.URL.Query().Get("kind"), r.URL.Query().Get("mode"), r.URL.Query().Get("status"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		items := make([]map[string]any, 0, len(result.Items))
		for _, run := range result.Items {
			items = append(items, map[string]any{"run": run})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": result.Page,
			"pageSize": result.PageSize, "total": result.Total, "hasNext": result.HasNext})
		return
	}
	runs, err := h.executions.ListRuns(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	items := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		items = append(items, map[string]any{"run": run})
	}
	writeJSON(w, http.StatusOK, items)
}

func positiveInteger(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func writeList(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeItem(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if isNil(value) {
		writeError(w, http.StatusNotFound, nil)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflectValue := reflect.ValueOf(value)
	return (reflectValue.Kind() == reflect.Ptr || reflectValue.Kind() == reflect.Interface) && reflectValue.IsNil()
}

func (h *Handler) CreateNetworkTopology(w http.ResponseWriter, r *http.Request) {
	var topology domain.NetworkTopology
	if !decode(w, r, &topology) {
		return
	}
	if err := h.topologies.Create(r.Context(), topology); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, topology)
}

func (h *Handler) GetNetworkTopology(w http.ResponseWriter, r *http.Request) {
	topology, err := h.topologies.Find(r.Context(), r.PathValue("topologyId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if topology == nil {
		writeError(w, http.StatusNotFound, nil)
		return
	}
	writeJSON(w, http.StatusOK, topology)
}

func (h *Handler) GetExecution(w http.ResponseWriter, r *http.Request) {
	run, err := h.executions.FindRun(r.Context(), r.PathValue("runId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, nil)
		return
	}
	tasks, err := h.executions.ListTasks(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	transfers, err := h.executions.ListTransfers(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	handles, err := h.executions.ListHandles(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	events, err := h.executions.ListEvents(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	response := map[string]any{
		"run": run, "activities": tasks, "dataTransfers": transfers,
		"handles": handles, "events": events,
	}
	if h.data != nil {
		instances, dataErr := h.data.ListInstances(r.Context(), run.ID)
		if dataErr != nil {
			writeError(w, http.StatusInternalServerError, dataErr)
			return
		}
		locations, locationErr := h.data.ListLocations(r.Context(), run.ID)
		if locationErr != nil {
			writeError(w, http.StatusInternalServerError, locationErr)
			return
		}
		response["dataObjects"], response["dataLocations"] = instances, locations
		materializations, materializationErr := h.data.ListArtifactMaterializations(r.Context(), run.ID)
		if materializationErr != nil {
			writeError(w, http.StatusInternalServerError, materializationErr)
			return
		}
		response["artifactMaterializations"] = materializations
		transferRuns, transferErr := h.data.ListArtifactTransferRuns(r.Context(), run.ID)
		if transferErr != nil {
			writeError(w, http.StatusInternalServerError, transferErr)
			return
		}
		response["artifactTransferRuns"] = transferRuns
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListArtifactLocations(w http.ResponseWriter, r *http.Request) {
	values, err := h.data.ListArtifactLocations(r.Context())
	writeList(w, values, err)
}

func (h *Handler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	values, err := h.data.ListArtifacts(r.Context())
	writeList(w, values, err)
}
func (h *Handler) ListArtifactMaterializations(w http.ResponseWriter, r *http.Request) {
	values, err := h.data.ListArtifactMaterializations(r.Context(), r.URL.Query().Get("runId"))
	writeList(w, values, err)
}
func (h *Handler) SaveArtifactMaterialization(w http.ResponseWriter, r *http.Request) {
	var value domain.ArtifactMaterialization
	if !decode(w, r, &value) {
		return
	}
	if err := h.data.SaveArtifactMaterialization(r.Context(), value); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

// SaveBuildContext records metadata for bytes already uploaded to the artifact
// store. It deliberately accepts no local path: browser paths are never build
// contexts on the server.
func (h *Handler) SaveBuildContext(w http.ResponseWriter, r *http.Request) {
	if h.build != nil && strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, h.build.MaxUploadBytes()+1<<20)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, err)
			return
		}
		file, _, err := r.FormFile("context")
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("multipart field context is required: %w", err))
			return
		}
		defer file.Close()
		value, err := h.build.Upload(r.Context(), file)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err)
			return
		}
		writeJSON(w, http.StatusCreated, value)
		return
	}
	var value domain.BuildContextArtifact
	if !decode(w, r, &value) {
		return
	}
	if value.Digest == "" || value.StorageURI == "" || value.SizeBytes < 1 {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("digest, storageUri and positive sizeBytes are required"))
		return
	}
	if err := h.data.SaveBuildContext(r.Context(), value); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func (h *Handler) CreateArtifactBuild(w http.ResponseWriter, r *http.Request) {
	var value domain.ArtifactBuild
	if !decode(w, r, &value) {
		return
	}
	if value.ID == "" || value.ArtifactVersionID == "" || value.ContextDigest == "" || value.RecipeDigest == "" || value.CacheKey == "" {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("id, artifactVersionId, contextDigest, recipeDigest and cacheKey are required"))
		return
	}
	if existing, err := h.data.FindArtifactBuildByCacheKey(r.Context(), value.CacheKey); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	} else if existing != nil {
		writeJSON(w, http.StatusOK, existing)
		return
	}
	if contextArtifact, err := h.data.FindBuildContext(r.Context(), value.ContextDigest); err != nil || contextArtifact == nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("build context %q is not uploaded", value.ContextDigest))
		return
	}
	if err := h.data.SaveArtifactBuild(r.Context(), value); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

// RegisterDockerArtifact creates an immutable catalog version and a SIF build
// specification. The actual registry pull happens only when its build run is
// started, never in the browser.
func (h *Handler) RegisterDockerArtifact(w http.ResponseWriter, r *http.Request) {
	var input DockerArtifactRequest
	if !decode(w, r, &input) {
		return
	}
	input.ArtifactID = strings.TrimSpace(input.ArtifactID)
	input.Version = strings.TrimSpace(input.Version)
	image := strings.TrimPrefix(strings.TrimSpace(input.Image), "docker://")
	if input.ArtifactID == "" || input.Version == "" || image == "" || strings.ContainsAny(image, " \t\n\r") {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("artifactId, version and a valid Docker image are required"))
		return
	}
	if input.Architecture == "" {
		input.Architecture = "amd64"
	}
	now := time.Now().UTC().UnixNano()
	version := domain.ArtifactVersion{ID: fmt.Sprintf("artifact-version-%d", now), ArtifactID: input.ArtifactID, Version: input.Version, Scope: domain.CatalogScope("system")}
	if err := h.data.RegisterArtifactVersion(r.Context(), version); err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("register artifact version: %w", err))
		return
	}
	sum := sha256.Sum256([]byte("docker://" + image))
	digest := "sha256:" + hex.EncodeToString(sum[:])
	build := domain.ArtifactBuild{ID: fmt.Sprintf("build-%d", now), ArtifactVersionID: version.ID, SourceType: "docker-image", ContextDigest: digest, RecipePath: image, RecipeDigest: digest, TargetFormat: "sif", TargetOS: "linux", TargetArchitecture: input.Architecture, BuildArguments: "{}", CacheKey: "docker-image:" + digest + ":" + version.ID}
	if err := h.data.SaveArtifactBuild(r.Context(), build); err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("create Docker artifact build: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"artifact": version, "build": build})
}
func (h *Handler) GetArtifactBuild(w http.ResponseWriter, r *http.Request) {
	value, err := h.data.FindArtifactBuild(r.Context(), r.PathValue("buildId"))
	writeItem(w, value, err)
}
func (h *Handler) ListArtifactBuilds(w http.ResponseWriter, r *http.Request) {
	values, err := h.data.ListArtifactBuilds(r.Context(), r.PathValue("artifactId"))
	writeList(w, values, err)
}
func (h *Handler) ListBuildRuns(w http.ResponseWriter, r *http.Request) {
	values, err := h.data.ListBuildRuns(r.Context(), r.PathValue("buildId"))
	writeList(w, values, err)
}
func (h *Handler) GetBuildRun(w http.ResponseWriter, r *http.Request) {
	value, err := h.data.FindBuildRun(r.Context(), r.PathValue("runId"))
	writeItem(w, value, err)
}

func (h *Handler) StreamBuildOutput(w http.ResponseWriter, r *http.Request) {
	if h.build == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("build service is unavailable"))
		return
	}
	file, name, err := h.build.OpenOutput(r.Context(), r.PathValue("runId"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/vnd.sylabs.sif")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = io.Copy(w, file)
}
func (h *Handler) StartArtifactBuildRun(w http.ResponseWriter, r *http.Request) {
	if h.build == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("build service is unavailable"))
		return
	}
	spec, err := h.data.FindArtifactBuild(r.Context(), r.PathValue("buildId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if spec == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("artifact build not found"))
		return
	}
	run, err := h.build.Start(r.Context(), *spec)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (h *Handler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var definition domain.EnvironmentDefinition
	if !decode(w, r, &definition) {
		return
	}
	if err := h.environments.Create(r.Context(), definition); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, definition)
}

func (h *Handler) ReplaceEnvironment(w http.ResponseWriter, r *http.Request) {
	var definition domain.EnvironmentDefinition
	if !decode(w, r, &definition) {
		return
	}
	if definition.Environment.ID != r.PathValue("environmentId") {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("environment id must match the request path"))
		return
	}
	if err := h.environments.Replace(r.Context(), definition); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, definition)
}

func (h *Handler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("environmentId")
	definition, err := h.environments.Find(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if definition == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
		return
	}
	if err := h.environments.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var request apirequests.Workflow
	if !decode(w, r, &request) {
		return
	}
	definition, err := request.Domain()
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if err := h.workflows.Create(r.Context(), definition); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, definition)
}

type DuplicateWorkflowRequest struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// DuplicateWorkflow creates an independent definition from the latest
// workflow version. A caller must provide a new name, which produces a new
// stable workflow and activity ID namespace.
func (h *Handler) DuplicateWorkflow(w http.ResponseWriter, r *http.Request) {
	var request DuplicateWorkflowRequest
	if !decode(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("duplicate workflow name is required"))
		return
	}
	source, err := h.workflows.Find(r.Context(), r.PathValue("workflowId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if source == nil {
		writeError(w, http.StatusNotFound, nil)
		return
	}
	copy := apirequests.FromDomain(*source)
	copy.Name = request.Name
	if request.Namespace != "" {
		copy.Spec.Namespace = request.Namespace
	}
	definition, err := copy.Domain()
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if err := h.workflows.Create(r.Context(), definition); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, definition)
}

type CreatePlanRequest struct {
	Plan            domain.SchedulePlan    `json:"plan"`
	Workflow        domain.WorkflowVersion `json:"workflow"`
	Resources       []domain.Resource      `json:"resources"`
	ExecutionScope  domain.ExecutionScope  `json:"executionScope"`
	NetworkTopology domain.NetworkTopology `json:"networkTopology"`
}

func (h *Handler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var request CreatePlanRequest
	if !decode(w, r, &request) {
		return
	}
	if request.Plan.NetworkTopologyID == "" {
		request.Plan.NetworkTopologyID = request.NetworkTopology.ID
	}
	if err := h.validator.Validate(request.Plan, request.Workflow, request.Resources, request.ExecutionScope, request.NetworkTopology); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if err := h.plans.Save(r.Context(), request.Plan); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, request.Plan)
}

func (h *Handler) GetPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := h.plans.Find(r.Context(), r.PathValue("planId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if plan == nil {
		writeError(w, http.StatusNotFound, nil)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) CreateExecution(w http.ResponseWriter, r *http.Request) {
	var request ports.ExecutionRequest
	if !decode(w, r, &request) {
		return
	}
	if err := h.resolveBuildPreparations(r.Context(), &request); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	resolveDirectOCIImages(&request.Workflow)
	request.Run.SchedulePlanID = request.Plan.ID
	payload, err := json.Marshal(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job, err := domainqueue.New(domainqueue.CategoryExecution, eventloop.EventExecutionRunRequested, payload, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job.AggregateType, job.AggregateID = "execution_run", request.Run.ID
	job.IdempotencyKey = "execution-run:" + request.Run.ID
	stored, err := h.events.Publish(r.Context(), job)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusAccepted, stored)
}

// Kubernetes accepts OCI references directly. Preserve the executable contract
// in the workflow while also filling the runtime-facing image field used by
// the Kubernetes adapter.
func resolveDirectOCIImages(workflow *domain.WorkflowVersion) {
	for index := range workflow.Activities {
		command := &workflow.Activities[index].Command
		if command.Image != "" || command.Executable == nil {
			continue
		}
		source := command.Executable.Source
		if source.Type == domain.ExecutableSourceOCI && source.Reference != "" {
			command.Image = source.Reference
		}
	}
}

func (h *Handler) resolveBuildPreparations(ctx context.Context, request *ports.ExecutionRequest) error {
	if h.data == nil {
		return nil
	}
	resources := make(map[string]domain.Resource, len(request.Resources))
	for _, resource := range request.Resources {
		resources[resource.ID] = resource
	}
	assignments := make(map[string]domain.PlanAssignment, len(request.Plan.Assignments))
	for _, assignment := range request.Plan.Assignments {
		assignments[assignment.ActivityID] = assignment
	}
	resolver := applicationbuild.OutputResolver{Catalog: h.data}
	for _, activity := range request.Workflow.Activities {
		if activity.Command.Executable == nil {
			continue
		}
		assignment, ok := assignments[activity.ID]
		if !ok {
			return fmt.Errorf("build activity %q has no plan assignment", activity.ID)
		}
		resource, ok := resources[assignment.ResourceID]
		if !ok {
			return fmt.Errorf("build activity %q has unknown resource %q", activity.ID, assignment.ResourceID)
		}
		var requirement domain.PreparationRequirement
		var required bool
		switch activity.Command.Executable.Source.Type {
		case domain.ExecutableSourceType("build"):
			var err error
			requirement, err = resolver.Preparation(ctx, activity.Command.Executable.Source.ArtifactBuildRef, activity.ID, resource, "")
			if err != nil {
				return err
			}
			required = true
		case domain.ExecutableSourceOCI:
			if executionRuntimeDriver(*request, assignment) != domain.RuntimeDriverSlurm {
				continue
			}
			var err error
			requirement, required, err = resolver.PreparationForDockerImage(ctx, activity.Command.Executable.Source.Reference, activity.ID, resource, "")
			if err != nil {
				return err
			}
		}
		if !required {
			continue
		}
		if err := h.configureGatewayArtifactTransfer(ctx, &requirement, resource); err != nil {
			return err
		}
		if request.PreparationRequirementsByActivity == nil {
			request.PreparationRequirementsByActivity = make(map[string]domain.PreparationRequirement)
		}
		request.PreparationRequirementsByActivity[activity.ID] = requirement
	}
	return nil
}

func (h *Handler) configureGatewayArtifactTransfer(ctx context.Context, requirement *domain.PreparationRequirement, resource domain.Resource) error {
	if requirement.Artifact == nil || requirement.ArtifactTransfer == nil {
		return nil
	}
	connections, ok := h.environments.(ports.ConnectionStore)
	if !ok {
		return nil
	}
	connectionID, _ := resource.Metadata["connectionId"].(string)
	if connectionID == "" {
		return nil
	}
	connection, err := connections.FindConnection(ctx, connectionID)
	if err != nil || connection == nil || connection.Type != domain.ConnectionSSH || connection.Endpoint == "" || connection.Username == "" {
		return err
	}
	root := path.Join("/home", connection.Username, ".akoflow", "artifacts")
	u := url.URL{Scheme: "ssh", User: url.User(connection.Username), Host: connection.Endpoint, Path: root}
	query := url.Values{}
	if identity := strings.TrimPrefix(connection.CredentialRef, "file:"); identity != "" {
		query.Set("identityFile", identity)
	}
	for _, key := range []string{"knownHostsFile", "proxyCommand"} {
		if value, _ := connection.Configuration[key].(string); value != "" {
			query.Set(key, value)
		}
	}
	u.RawQuery = query.Encode()
	// The transfer materializer stores each blob under its complete digest. The
	// resolved Slurm path must use that exact name, including the sha256 prefix.
	requirement.Artifact.DestinationPath = path.Join(root, requirement.Artifact.Digest)
	requirement.ArtifactTransfer.Strategy = domain.TransferSourcePush
	requirement.ArtifactTransfer.Destination.URI = u.String()
	requirement.ArtifactTransfer.Destination.Path = ""
	return nil
}

func executionRuntimeDriver(request ports.ExecutionRequest, assignment domain.PlanAssignment) domain.RuntimeDriver {
	mode := domain.RuntimeModeExecution
	if request.Run.Mode == domain.ExecutionModeSimulation {
		mode = domain.RuntimeModeSimulation
	}
	runtimes := make(map[string]domain.EnvironmentRuntime, len(request.Runtimes))
	for _, runtime := range request.Runtimes {
		runtimes[runtime.ID] = runtime
	}
	selected, _ := assignment.Metadata["runtimeId"].(string)
	for _, binding := range request.RuntimeBindings {
		if binding.ResourceID != assignment.ResourceID || !binding.Enabled {
			continue
		}
		if runtime, found := runtimes[binding.RuntimeID]; found && runtime.Mode == mode && (selected == "" || runtime.ID == selected) {
			return runtime.Driver
		}
	}
	return ""
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	contentType := normalizedContentType(r.Header.Get("Content-Type"))
	if contentType != "application/json" && !isYAML(contentType) {
		writeError(w, http.StatusUnsupportedMediaType, fmt.Errorf("request Content-Type must be application/json or application/yaml"))
		return false
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	if isYAML(contentType) {
		var document any
		if err := yaml.Unmarshal(payload, &document); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode YAML: %w", err))
			return false
		}
		payload, err = json.Marshal(document)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("normalize YAML: %w", err))
			return false
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func isYAML(contentType string) bool {
	return contentType == "application/yaml" || contentType == "application/x-yaml" || contentType == "text/yaml"
}

func normalizedContentType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	message := http.StatusText(status)
	if err != nil {
		message = err.Error()
	}
	writeJSON(w, status, map[string]string{"error": message})
}
