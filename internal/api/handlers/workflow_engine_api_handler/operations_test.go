package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
)

type storageNavigatorStub struct {
	err                                     error
	deleted, promotedData, promotedArtifact bool
}

func (s *storageNavigatorStub) List(context.Context, string) ([]domain.StorageResource, error) {
	return []domain.StorageResource{{ID: "storage"}}, s.err
}
func (s *storageNavigatorStub) Roots(context.Context, string) ([]domain.StorageBrowseRoot, error) {
	return []domain.StorageBrowseRoot{{Path: "/data"}}, s.err
}
func (s *storageNavigatorStub) Browse(_ context.Context, id string, request domain.BrowseRequest) (domain.BrowsePage, error) {
	return domain.BrowsePage{StorageID: id, Path: request.Path, Entries: []domain.FileEntry{{Path: request.Path + "/file"}}}, s.err
}
func (s *storageNavigatorStub) Stat(context.Context, string, string) (domain.FileEntry, error) {
	return domain.FileEntry{Name: "file.txt", Path: "/data/file.txt", Type: domain.FileEntryFile}, s.err
}
func (s *storageNavigatorStub) StartDownload(_ context.Context, storage, path, id string) (domain.DownloadRun, error) {
	return domain.DownloadRun{ID: id, StorageID: storage, Path: path, Status: domain.DownloadReady}, s.err
}
func (s *storageNavigatorStub) OpenDownload(context.Context, string) (io.ReadCloser, domain.FileEntry, error) {
	return io.NopCloser(strings.NewReader("payload")), domain.FileEntry{Name: "file.txt"}, s.err
}
func (s *storageNavigatorStub) Download(_ context.Context, id string) (*domain.DownloadRun, error) {
	return &domain.DownloadRun{ID: id}, s.err
}
func (s *storageNavigatorStub) Checksum(context.Context, string, string) (string, error) {
	return "sha256:abc", s.err
}
func (s *storageNavigatorStub) QueueCopy(_ context.Context, _, path, destination, id string) (domain.DownloadRun, error) {
	return domain.DownloadRun{ID: id, Path: path, Strategy: "copy:" + destination}, s.err
}
func (s *storageNavigatorStub) QueueArchive(_ context.Context, _, path, id string) (domain.DownloadRun, error) {
	return domain.DownloadRun{ID: id, Path: path, Strategy: "archive"}, s.err
}
func (s *storageNavigatorStub) PromoteData(context.Context, string, string, string, string, string, string) error {
	s.promotedData = true
	return s.err
}
func (s *storageNavigatorStub) PromoteArtifact(context.Context, string, string, string, string, string, string, string) error {
	s.promotedArtifact = true
	return s.err
}
func (s *storageNavigatorStub) IndexRuns(context.Context, string) ([]domain.IndexRun, error) {
	return []domain.IndexRun{{ID: "index"}}, s.err
}
func (s *storageNavigatorStub) StartIndex(_ context.Context, storage, id string) (domain.IndexRun, error) {
	return domain.IndexRun{ID: id, StorageID: storage}, s.err
}
func (s *storageNavigatorStub) Delete(context.Context, string, string) error {
	s.deleted = true
	return s.err
}

func callHandler(t *testing.T, method, target, body string, pathValues map[string]string, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range pathValues {
		request.SetPathValue(key, value)
	}
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}

func TestStorageHTTPHandlers(t *testing.T) {
	storage := &storageNavigatorStub{}
	h := &Handler{storage: storage}
	tests := []struct {
		name, method, target, body string
		values                     map[string]string
		handler                    http.HandlerFunc
		status                     int
		contains                   string
	}{
		{"list", http.MethodGet, "/?x=1", "", map[string]string{"environmentId": "env"}, h.ListStorages, 200, "storage"},
		{"roots", http.MethodGet, "/", "", map[string]string{"storageId": "storage"}, h.StorageRoots, 200, "/data"},
		{"browse", http.MethodGet, "/?path=/data&limit=5&cursor=x", "", map[string]string{"storageId": "storage"}, h.BrowseStorage, 200, "file"},
		{"stat", http.MethodGet, "/?path=/data/file.txt", "", map[string]string{"storageId": "storage"}, h.StatStorageEntry, 200, "file.txt"},
		{"download", http.MethodPost, "/", `{"path":"/data/file.txt","id":"download"}`, map[string]string{"storageId": "storage"}, h.CreateDownload, 201, "download"},
		{"stream", http.MethodGet, "/", "", map[string]string{"downloadId": "download"}, h.StreamDownload, 200, "payload"},
		{"get download", http.MethodGet, "/", "", map[string]string{"downloadId": "download"}, h.GetDownload, 200, "download"},
		{"checksum", http.MethodPost, "/", `{"path":"/data/file.txt"}`, map[string]string{"storageId": "storage"}, h.ChecksumStorageEntry, 200, "sha256:abc"},
		{"delete", http.MethodDelete, "/?path=/data/file.txt", "", map[string]string{"storageId": "storage"}, h.DeleteStorageEntry, 204, ""},
		{"copy", http.MethodPost, "/", `{"path":"/data/file.txt","destinationStorageId":"other","id":"copy"}`, map[string]string{"storageId": "storage"}, h.CopyStorageEntry, 202, "copy:other"},
		{"archive", http.MethodPost, "/", `{"path":"/data/tree","id":"archive"}`, map[string]string{"storageId": "storage"}, h.ArchiveStorageDirectory, 202, "archive"},
		{"promote data", http.MethodPost, "/", `{"Path":"/data/file.txt","ID":"data"}`, map[string]string{"storageId": "storage"}, h.PromoteStorageData, 201, "data"},
		{"promote artifact", http.MethodPost, "/", `{"Path":"/data/tool.sif","ID":"artifact"}`, map[string]string{"storageId": "storage"}, h.PromoteStorageArtifact, 201, "artifact"},
		{"index list", http.MethodGet, "/", "", map[string]string{"storageId": "storage"}, h.ListStorageIndexRuns, 200, "index"},
		{"index start", http.MethodPost, "/", `{"id":"index-2"}`, map[string]string{"storageId": "storage"}, h.StartStorageIndex, 202, "index-2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := callHandler(t, test.method, test.target, test.body, test.values, test.handler)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
	if !storage.deleted || !storage.promotedData || !storage.promotedArtifact {
		t.Fatalf("mutations = %#v", storage)
	}
}

func TestStorageHTTPHandlersReportUnavailableAndOperationErrors(t *testing.T) {
	empty := &Handler{}
	for _, handler := range []http.HandlerFunc{empty.ListStorages, empty.StorageRoots, empty.BrowseStorage, empty.StatStorageEntry, empty.CreateDownload, empty.StreamDownload, empty.GetDownload, empty.ChecksumStorageEntry, empty.DeleteStorageEntry} {
		if response := callHandler(t, http.MethodGet, "/", "", nil, handler); response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unavailable = %d", response.Code)
		}
	}
	storage := &storageNavigatorStub{err: fmt.Errorf("storage error")}
	h := &Handler{storage: storage}
	calls := []struct {
		method, body string
		handler      http.HandlerFunc
	}{
		{http.MethodGet, "", h.BrowseStorage}, {http.MethodGet, "", h.StatStorageEntry}, {http.MethodPost, `{"path":"x"}`, h.CreateDownload}, {http.MethodGet, "", h.StreamDownload}, {http.MethodGet, "", h.GetDownload}, {http.MethodPost, `{"path":"x"}`, h.ChecksumStorageEntry}, {http.MethodDelete, "", h.DeleteStorageEntry}, {http.MethodPost, `{"path":"x"}`, h.CopyStorageEntry}, {http.MethodPost, `{"path":"x"}`, h.ArchiveStorageDirectory}, {http.MethodPost, `{"Path":"x"}`, h.PromoteStorageData}, {http.MethodPost, `{"Path":"x"}`, h.PromoteStorageArtifact}, {http.MethodPost, `{}`, h.StartStorageIndex},
	}
	for _, call := range calls {
		if response := callHandler(t, call.method, "/?path=x", call.body, map[string]string{"storageId": "storage", "downloadId": "download"}, call.handler); response.Code < 400 {
			t.Fatalf("expected error from %T, got %d", call.handler, response.Code)
		}
	}
}

type consoleStub struct{ err error }

func (s consoleStub) ExecuteCommand(context.Context, domainconsole.Request) (domainconsole.Command, error) {
	return domainconsole.Command{ID: "command"}, s.err
}
func (s consoleStub) ListCommands(context.Context, int) ([]domainconsole.Command, error) {
	return []domainconsole.Command{{ID: "command"}}, s.err
}

type terminalStub struct {
	err      error
	streamed bool
}

func (s *terminalStub) OpenSession(context.Context, domainconsole.SessionRequest) (domainconsole.Session, error) {
	return domainconsole.Session{ID: "session"}, s.err
}
func (s *terminalStub) ListSessions(context.Context) ([]domainconsole.Session, error) {
	return []domainconsole.Session{{ID: "session"}}, s.err
}
func (s *terminalStub) CloseSession(context.Context, string) error { return s.err }
func (s *terminalStub) SessionLog(context.Context, string) ([]byte, error) {
	return []byte("terminal log"), s.err
}
func (s *terminalStub) StreamSession(w http.ResponseWriter, _ *http.Request, _ string) {
	s.streamed = true
	_, _ = w.Write([]byte("stream"))
}

type auditStub struct {
	filter domainaudit.Filter
	err    error
}

func (s *auditStub) RecordAuditEvent(context.Context, domainaudit.Event) error { return s.err }
func (s *auditStub) ListAuditEvents(_ context.Context, filter domainaudit.Filter) ([]domainaudit.Event, error) {
	s.filter = filter
	return []domainaudit.Event{{ID: "audit"}}, s.err
}

func TestConsoleTerminalAndAuditHTTPHandlers(t *testing.T) {
	terminal, audit := &terminalStub{}, &auditStub{}
	h := &Handler{console: consoleStub{}, terminal: terminal, audit: audit}
	tests := []struct {
		method, target, body string
		values               map[string]string
		handler              http.HandlerFunc
		status               int
		contains             string
	}{
		{http.MethodPost, "/", `{"resourceId":"resource","command":"hostname"}`, nil, h.ExecuteConsoleCommand, 201, "command"},
		{http.MethodGet, "/?limit=5", "", nil, h.ListConsoleCommands, 200, "command"},
		{http.MethodPost, "/", `{"resourceId":"resource"}`, nil, h.OpenConsoleSession, 201, "session"},
		{http.MethodGet, "/", "", map[string]string{"sessionId": "session"}, h.StreamConsoleSession, 200, "stream"},
		{http.MethodGet, "/", "", nil, h.ListConsoleSessions, 200, "session"},
		{http.MethodDelete, "/", "", map[string]string{"sessionId": "session"}, h.CloseConsoleSession, 204, ""},
		{http.MethodGet, "/", "", map[string]string{"sessionId": "session"}, h.ExportConsoleSessionLog, 200, "terminal log"},
		{http.MethodGet, "/?limit=3&eventType=run&environmentId=env&outcome=succeeded", "", nil, h.ListAuditEvents, 200, "audit"},
	}
	for _, test := range tests {
		response := callHandler(t, test.method, test.target, test.body, test.values, test.handler)
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
			t.Fatalf("response = %d %q", response.Code, response.Body.String())
		}
	}
	if !terminal.streamed || audit.filter.Limit != 3 || audit.filter.EnvironmentID != "env" {
		t.Fatalf("terminal/audit = %v %#v", terminal.streamed, audit.filter)
	}
}

func TestConsoleTerminalAndAuditUnavailableResponses(t *testing.T) {
	h := &Handler{}
	for _, call := range []struct {
		method  string
		handler http.HandlerFunc
		status  int
	}{{http.MethodPost, h.ExecuteConsoleCommand, 503}, {http.MethodGet, h.ListConsoleCommands, 503}, {http.MethodPost, h.OpenConsoleSession, 503}, {http.MethodGet, h.StreamConsoleSession, 503}, {http.MethodDelete, h.CloseConsoleSession, 503}, {http.MethodGet, h.ExportConsoleSessionLog, 503}, {http.MethodGet, h.ListAuditEvents, 503}} {
		if response := callHandler(t, call.method, "/", `{}`, nil, call.handler); response.Code != call.status {
			t.Fatalf("unavailable = %d", response.Code)
		}
	}
	if response := callHandler(t, http.MethodGet, "/", "", nil, h.ListConsoleSessions); response.Code != 200 || response.Body.String() != "[]\n" {
		t.Fatalf("empty sessions = %d %q", response.Code, response.Body.String())
	}
}
