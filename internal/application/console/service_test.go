package console

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
	"github.com/gorilla/websocket"
)

type environmentCatalogStub struct {
	ports.EnvironmentCatalog
	definitions []domain.EnvironmentDefinition
	err         error
}

func (s environmentCatalogStub) List(context.Context) ([]domain.EnvironmentDefinition, error) {
	return s.definitions, s.err
}

type resourceInventoryStub struct {
	ports.ResourceInventory
	resource *domain.Resource
	err      error
}

func (s resourceInventoryStub) FindByID(context.Context, string) (*domain.Resource, error) {
	return s.resource, s.err
}

type commandStoreStub struct {
	commands []domainconsole.Command
	saveErr  error
}

func (s *commandStoreStub) SaveConsoleCommand(_ context.Context, command domainconsole.Command) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.commands = append(s.commands, command)
	return nil
}
func (s *commandStoreStub) ListConsoleCommands(context.Context, int) ([]domainconsole.Command, error) {
	return s.commands, nil
}
func (s *commandStoreStub) FindConsoleCommand(context.Context, string) (*domainconsole.Command, error) {
	return nil, nil
}

type commandRunnerStub struct {
	stdout, stderr string
	exit           int
	external       string
	err            error
}

func (s commandRunnerStub) RunConsoleCommand(context.Context, domain.EnvironmentConnection, domain.Resource, domainconsole.Command) (string, string, int, string, error) {
	return s.stdout, s.stderr, s.exit, s.external, s.err
}

type auditStoreStub struct{ events []domainaudit.Event }

func (s *auditStoreStub) RecordAuditEvent(_ context.Context, event domainaudit.Event) error {
	s.events = append(s.events, event)
	return nil
}
func (s *auditStoreStub) ListAuditEvents(context.Context, domainaudit.Filter) ([]domainaudit.Event, error) {
	return s.events, nil
}

func consoleTarget() (domain.Resource, domain.EnvironmentDefinition) {
	resource := domain.Resource{ID: "machine", EnvironmentVersionID: "version"}
	definition := domain.EnvironmentDefinition{
		Environment:     domain.Environment{ID: "environment"},
		Version:         domain.EnvironmentVersion{ID: "version"},
		Runtimes:        []domain.EnvironmentRuntime{{ID: "runtime", Configuration: map[string]any{"connectionId": "connection"}}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: resource.ID, RuntimeID: "runtime", Enabled: true}},
		Connections:     []domain.EnvironmentConnection{{ID: "connection"}},
	}
	return resource, definition
}

func TestCommandControllerExecutesAndAuditsCommands(t *testing.T) {
	resource, definition := consoleTarget()
	store, audit := &commandStoreStub{}, &auditStoreStub{}
	controller := NewCommandController(environmentCatalogStub{definitions: []domain.EnvironmentDefinition{definition}}, resourceInventoryStub{resource: &resource}, store, commandRunnerStub{stdout: "done", stderr: "warning", exit: 2, external: "job-1"}, audit)
	command, err := controller.ExecuteCommand(context.Background(), domainconsole.Request{ResourceID: resource.ID, ActorID: "user", Command: "hostname"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Status != domainconsole.CommandCompleted || command.TimeoutSeconds != 30 || command.Stdout != "done" || *command.ExitCode != 2 {
		t.Fatalf("command = %#v", command)
	}
	if len(store.commands) != 2 || len(audit.events) != 2 {
		t.Fatalf("saved/audited = %d/%d", len(store.commands), len(audit.events))
	}
	listed, err := controller.ListCommands(context.Background(), 10)
	if err != nil || len(listed) != 2 {
		t.Fatalf("ListCommands() = %#v, %v", listed, err)
	}
}

func TestCommandControllerFailureAndValidation(t *testing.T) {
	resource, definition := consoleTarget()
	store := &commandStoreStub{}
	controller := NewCommandController(environmentCatalogStub{definitions: []domain.EnvironmentDefinition{definition}}, resourceInventoryStub{resource: &resource}, store, commandRunnerStub{exit: 255, err: fmt.Errorf("ssh failed")}, nil)
	command, err := controller.ExecuteCommand(context.Background(), domainconsole.Request{ResourceID: resource.ID, Command: "false", TimeoutSeconds: 2})
	if err != nil || command.Status != domainconsole.CommandFailed || command.Failure != "ssh failed" {
		t.Fatalf("failure = %#v, %v", command, err)
	}
	for _, request := range []domainconsole.Request{{}, {ResourceID: resource.ID, Command: "x", TimeoutSeconds: 3601}} {
		if _, err = controller.ExecuteCommand(context.Background(), request); err == nil {
			t.Fatalf("expected validation error for %#v", request)
		}
	}
	missing := NewCommandController(environmentCatalogStub{}, resourceInventoryStub{}, store, commandRunnerStub{}, nil)
	if _, err = missing.ExecuteCommand(context.Background(), domainconsole.Request{ResourceID: "missing", Command: "x"}); err == nil {
		t.Fatal("expected resource error")
	}
	unbound := NewCommandController(environmentCatalogStub{definitions: []domain.EnvironmentDefinition{definition}}, resourceInventoryStub{resource: &domain.Resource{ID: "other", EnvironmentVersionID: "version"}}, store, commandRunnerStub{}, nil)
	if _, err = unbound.ExecuteCommand(context.Background(), domainconsole.Request{ResourceID: "other", Command: "x"}); err == nil {
		t.Fatal("expected target error")
	}
}

type interactiveRunnerStub struct {
	terminal ports.InteractiveTerminal
	err      error
}

func (s interactiveRunnerStub) StartInteractive(context.Context, domain.EnvironmentConnection, domain.Resource) (ports.InteractiveTerminal, error) {
	return s.terminal, s.err
}

type terminalStub struct {
	mu            sync.Mutex
	reads         chan []byte
	closed        chan struct{}
	closeOnce     sync.Once
	writes        []byte
	rows, columns uint16
}

func newTerminalStub() *terminalStub {
	return &terminalStub{reads: make(chan []byte, 2), closed: make(chan struct{})}
}
func (s *terminalStub) Read(p []byte) (int, error) {
	select {
	case data := <-s.reads:
		return copy(p, data), nil
	case <-s.closed:
		return 0, io.EOF
	}
}
func (s *terminalStub) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, p...)
	return len(p), nil
}
func (s *terminalStub) Resize(rows, columns uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows, s.columns = rows, columns
	return nil
}
func (s *terminalStub) Close() error { s.closeOnce.Do(func() { close(s.closed) }); return nil }

type sessionLogStub struct {
	mu       sync.Mutex
	sessions []domainconsole.Session
	data     map[string][]byte
	saveErr  error
}

func (s *sessionLogStub) SaveConsoleSession(_ context.Context, session domainconsole.Session) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = append(s.sessions, session)
	return nil
}
func (s *sessionLogStub) AppendConsoleSessionLog(_ context.Context, id, _ string, data []byte, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[id] = append(s.data[id], data...)
	return nil
}
func (s *sessionLogStub) ReadConsoleSessionLog(_ context.Context, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.data[id]...), nil
}

func TestTerminalControllerLifecycleLoggingAndResize(t *testing.T) {
	resource, definition := consoleTarget()
	commands := NewCommandController(environmentCatalogStub{definitions: []domain.EnvironmentDefinition{definition}}, resourceInventoryStub{resource: &resource}, &commandStoreStub{}, commandRunnerStub{}, nil)
	terminal, logs, audit := newTerminalStub(), &sessionLogStub{}, &auditStoreStub{}
	controller := NewTerminalController(commands, interactiveRunnerStub{terminal: terminal}, audit, logs)
	session, err := controller.OpenSession(context.Background(), domainconsole.SessionRequest{ResourceID: resource.ID, ActorID: "user"})
	if err != nil || session.Status != domainconsole.SessionConnected {
		t.Fatalf("OpenSession() = %#v, %v", session, err)
	}
	terminal.reads <- []byte("hello")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		value, _ := controller.SessionLog(context.Background(), session.ID)
		if string(value) == "hello" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	value, err := controller.SessionLog(context.Background(), session.ID)
	if err != nil || string(value) != "hello" {
		t.Fatalf("SessionLog() = %q, %v", value, err)
	}
	items, _ := controller.ListSessions(context.Background())
	if len(items) != 1 {
		t.Fatalf("sessions = %#v", items)
	}
	controller.mu.RLock()
	live := controller.sessions[session.ID]
	controller.mu.RUnlock()
	if !controller.resize(live, []byte(`{"type":"resize","rows":40,"columns":120}`)) || controller.resize(live, []byte(`{"type":"input"}`)) {
		t.Fatal("unexpected resize parsing")
	}
	if terminal.rows != 40 || terminal.columns != 120 {
		t.Fatalf("size = %dx%d", terminal.rows, terminal.columns)
	}
	if err = controller.CloseSession(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	if err = controller.CloseSession(context.Background(), session.ID); err == nil {
		t.Fatal("expected closed session to disappear")
	}
	archived, err := controller.SessionLog(context.Background(), session.ID)
	if err != nil || string(archived) != "hello" {
		t.Fatalf("archived log = %q, %v", archived, err)
	}
}

func TestTerminalControllerOpenFailures(t *testing.T) {
	resource, definition := consoleTarget()
	commands := NewCommandController(environmentCatalogStub{definitions: []domain.EnvironmentDefinition{definition}}, resourceInventoryStub{resource: &resource}, &commandStoreStub{}, commandRunnerStub{}, nil)
	controller := NewTerminalController(commands, interactiveRunnerStub{err: fmt.Errorf("no tty")}, nil, &sessionLogStub{})
	if _, err := controller.OpenSession(context.Background(), domainconsole.SessionRequest{}); err == nil {
		t.Fatal("expected resource validation")
	}
	session, err := controller.OpenSession(context.Background(), domainconsole.SessionRequest{ResourceID: resource.ID})
	if err == nil || session.Status != domainconsole.SessionFailed {
		t.Fatalf("failed session = %#v, %v", session, err)
	}
	logs := &sessionLogStub{saveErr: fmt.Errorf("database")}
	controller = NewTerminalController(commands, interactiveRunnerStub{terminal: newTerminalStub()}, nil, logs)
	if _, err = controller.OpenSession(context.Background(), domainconsole.SessionRequest{ResourceID: resource.ID}); err == nil {
		t.Fatal("expected persistence error")
	}
}

func TestTerminalControllerStreamsWebSocketTraffic(t *testing.T) {
	terminal := newTerminalStub()
	controller := NewTerminalController(nil, nil, nil, nil)
	live := &liveSession{value: domainconsole.Session{ID: "session"}, terminal: terminal, log: []byte("history"), clients: map[*terminalClient]struct{}{}}
	controller.sessions[live.value.ID] = live
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller.StreamSession(w, r, live.value.ID)
	}))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	connection, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, history, err := connection.ReadMessage()
	if err != nil || string(history) != "history" {
		t.Fatalf("history = %q, %v", history, err)
	}
	if err = connection.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","rows":24,"columns":80}`)); err != nil {
		t.Fatal(err)
	}
	if err = connection.WriteMessage(websocket.BinaryMessage, []byte("ls\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		terminal.mu.Lock()
		written := string(terminal.writes)
		terminal.mu.Unlock()
		if written == "ls\n" && terminal.rows == 24 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	terminal.mu.Lock()
	written, rows := string(terminal.writes), terminal.rows
	terminal.mu.Unlock()
	if written != "ls\n" || rows != 24 {
		t.Fatalf("terminal traffic = %q, rows %d", written, rows)
	}
	controller.publish(live, []byte("result"))
	_, output, err := connection.ReadMessage()
	if err != nil || string(output) != "result" {
		t.Fatalf("output = %q, %v", output, err)
	}
	_ = connection.Close()
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		live.mu.Lock()
		count := len(live.clients)
		live.mu.Unlock()
		if count == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("websocket client was not removed")
}

func TestTerminalFailureRemovesSession(t *testing.T) {
	terminal := newTerminalStub()
	controller := NewTerminalController(nil, nil, nil, &sessionLogStub{})
	live := &liveSession{value: domainconsole.Session{ID: "failed"}, terminal: terminal, clients: map[*terminalClient]struct{}{}}
	controller.sessions[live.value.ID] = live
	controller.failSession(live, fmt.Errorf("connection lost"))
	if live.value.Status != domainconsole.SessionFailed || live.value.Failure != "connection lost" {
		t.Fatalf("session = %#v", live.value)
	}
	if _, ok := controller.sessions[live.value.ID]; ok {
		t.Fatal("failed session still registered")
	}
}
