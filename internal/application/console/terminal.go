package console

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
	"github.com/UFFeScience/akoflow/internal/provider"
	"github.com/gorilla/websocket"
)

const maxTerminalLogBytes = 1 << 20

type TerminalController struct {
	commands *CommandController
	runner   ports.InteractiveConsoleRunner
	audit    ports.AuditStore
	logs     ports.ConsoleSessionLogStore
	mu       sync.RWMutex
	sessions map[string]*liveSession
}
type liveSession struct {
	value       domainconsole.Session
	terminal    ports.InteractiveTerminal
	environment string
	mu          sync.Mutex
	log         []byte
	clients     map[*terminalClient]struct{}
	closeOnce   sync.Once
}
type terminalClient struct {
	connection *websocket.Conn
	mu         sync.Mutex
}

var _ ports.InteractiveConsole = (*TerminalController)(nil)

func NewTerminalController(commands *CommandController, runner ports.InteractiveConsoleRunner, audit ports.AuditStore, logs ports.ConsoleSessionLogStore) *TerminalController {
	return &TerminalController{commands: commands, runner: runner, audit: audit, logs: logs, sessions: map[string]*liveSession{}}
}

func (c *TerminalController) OpenSession(ctx context.Context, request domainconsole.SessionRequest) (domainconsole.Session, error) {
	if request.ResourceID == "" {
		return domainconsole.Session{}, fmt.Errorf("resourceId is required")
	}
	resource, err := c.commands.resources.FindByID(ctx, request.ResourceID)
	if err != nil || resource == nil {
		if err == nil {
			err = fmt.Errorf("resource %q was not found", request.ResourceID)
		}
		return domainconsole.Session{}, err
	}
	runtime, connection, environmentID, err := c.commands.resolveTarget(ctx, *resource)
	if err != nil {
		return domainconsole.Session{}, err
	}
	now := time.Now().UTC()
	value := domainconsole.Session{ID: provider.NewID("console-session"), ResourceID: resource.ID, RuntimeID: runtime.ID, ConnectionID: connection.ID, ActorID: request.ActorID, Status: domainconsole.SessionStarting, CreatedAt: now}
	if c.logs != nil {
		if err := c.logs.SaveConsoleSession(ctx, value); err != nil {
			return domainconsole.Session{}, fmt.Errorf("persist interactive session: %w", err)
		}
	}
	c.record(value, environmentID, "console.session.started", domainaudit.OutcomeStarted, "Interactive terminal requested")
	terminal, err := c.runner.StartInteractive(ctx, connection, *resource)
	if err != nil {
		value.Status, value.Failure = domainconsole.SessionFailed, err.Error()
		finished := time.Now().UTC()
		value.FinishedAt = &finished
		if c.logs != nil {
			_ = c.logs.SaveConsoleSession(context.Background(), value)
		}
		c.record(value, environmentID, "console.session.failed", domainaudit.OutcomeFailed, value.Failure)
		return value, err
	}
	connected := time.Now().UTC()
	value.Status, value.ConnectedAt = domainconsole.SessionConnected, &connected
	if c.logs != nil {
		if err := c.logs.SaveConsoleSession(ctx, value); err != nil {
			_ = terminal.Close()
			return domainconsole.Session{}, fmt.Errorf("persist connected interactive session: %w", err)
		}
	}
	session := &liveSession{value: value, terminal: terminal, environment: environmentID, clients: map[*terminalClient]struct{}{}}
	c.mu.Lock()
	c.sessions[value.ID] = session
	c.mu.Unlock()
	go c.readTerminal(session)
	c.record(value, environmentID, "console.session.connected", domainaudit.OutcomeSucceeded, "Interactive terminal connected")
	return value, nil
}

func (c *TerminalController) ListSessions(_ context.Context) ([]domainconsole.Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	items := make([]domainconsole.Session, 0, len(c.sessions))
	for _, session := range c.sessions {
		items = append(items, session.value)
	}
	return items, nil
}
func (c *TerminalController) CloseSession(_ context.Context, sessionID string) error {
	c.mu.RLock()
	session := c.sessions[sessionID]
	c.mu.RUnlock()
	if session == nil {
		return fmt.Errorf("console session was not found")
	}
	c.closeSession(session)
	return nil
}
func (c *TerminalController) SessionLog(_ context.Context, sessionID string) ([]byte, error) {
	c.mu.RLock()
	session := c.sessions[sessionID]
	c.mu.RUnlock()
	if session == nil {
		if c.logs != nil {
			return c.logs.ReadConsoleSessionLog(context.Background(), sessionID)
		}
		return nil, fmt.Errorf("console session was not found")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return append([]byte(nil), session.log...), nil
}

func (c *TerminalController) readTerminal(session *liveSession) {
	buffer := make([]byte, 4096)
	for {
		count, err := session.terminal.Read(buffer)
		if count > 0 {
			c.publish(session, buffer[:count])
		}
		if err != nil {
			if err == io.EOF {
				c.closeSession(session)
			} else {
				c.failSession(session, fmt.Errorf("interactive terminal disconnected: %w", err))
			}
			return
		}
	}
}
func (c *TerminalController) publish(session *liveSession, payload []byte) {
	c.appendLog(session.value.ID, "output", payload)
	session.mu.Lock()
	session.log = append(session.log, payload...)
	if len(session.log) > maxTerminalLogBytes {
		session.log = append([]byte(nil), session.log[len(session.log)-maxTerminalLogBytes:]...)
	}
	clients := make([]*terminalClient, 0, len(session.clients))
	for client := range session.clients {
		clients = append(clients, client)
	}
	session.mu.Unlock()
	for _, client := range clients {
		client.mu.Lock()
		err := client.connection.WriteMessage(websocket.BinaryMessage, payload)
		client.mu.Unlock()
		if err != nil {
			c.removeClient(session, client)
		}
	}
}

var terminalUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

func (c *TerminalController) StreamSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	c.mu.RLock()
	session := c.sessions[sessionID]
	c.mu.RUnlock()
	if session == nil {
		http.Error(w, "console session was not found", http.StatusNotFound)
		return
	}
	connection, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	client := &terminalClient{connection: connection}
	session.mu.Lock()
	session.clients[client] = struct{}{}
	history := append([]byte(nil), session.log...)
	session.mu.Unlock()
	if len(history) > 0 {
		client.mu.Lock()
		_ = connection.WriteMessage(websocket.BinaryMessage, history)
		client.mu.Unlock()
	}
	defer c.removeClient(session, client)
	for {
		messageType, payload, err := connection.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		if messageType == websocket.TextMessage && c.resize(session, payload) {
			continue
		}
		if _, err := session.terminal.Write(payload); err != nil {
			c.closeSession(session)
			return
		}
		c.appendLog(session.value.ID, "input", payload)
	}
}

func (c *TerminalController) appendLog(sessionID, direction string, payload []byte) {
	if c.logs != nil && len(payload) > 0 {
		_ = c.logs.AppendConsoleSessionLog(context.Background(), sessionID, direction, append([]byte(nil), payload...), time.Now().UTC())
	}
}
func (c *TerminalController) removeClient(session *liveSession, client *terminalClient) {
	session.mu.Lock()
	delete(session.clients, client)
	session.mu.Unlock()
}
func (c *TerminalController) resize(session *liveSession, payload []byte) bool {
	var message struct {
		Type    string `json:"type"`
		Rows    uint16 `json:"rows"`
		Columns uint16 `json:"columns"`
	}
	if json.Unmarshal(payload, &message) != nil || message.Type != "resize" {
		return false
	}
	if message.Rows > 0 && message.Columns > 0 {
		_ = session.terminal.Resize(message.Rows, message.Columns)
	}
	return true
}
func (c *TerminalController) closeSession(session *liveSession) {
	c.finishSession(session, domainconsole.SessionClosed, "", "console.session.closed", domainaudit.OutcomeSucceeded, "Interactive terminal closed")
}

func (c *TerminalController) failSession(session *liveSession, err error) {
	c.finishSession(session, domainconsole.SessionFailed, err.Error(), "console.session.failed", domainaudit.OutcomeFailed, err.Error())
}

func (c *TerminalController) finishSession(
	session *liveSession,
	status domainconsole.SessionStatus,
	failure, eventType string,
	outcome domainaudit.Outcome,
	summary string,
) {
	session.closeOnce.Do(func() {
		_ = session.terminal.Close()
		finished := time.Now().UTC()
		session.value.Status, session.value.Failure, session.value.FinishedAt = status, failure, &finished
		if c.logs != nil {
			_ = c.logs.SaveConsoleSession(context.Background(), session.value)
		}
		c.mu.Lock()
		delete(c.sessions, session.value.ID)
		c.mu.Unlock()
		c.record(session.value, session.environment, eventType, outcome, summary)
	})
}
func (c *TerminalController) record(value domainconsole.Session, environmentID, eventType string, outcome domainaudit.Outcome, summary string) {
	if c.audit == nil {
		return
	}
	_ = c.audit.RecordAuditEvent(context.Background(), domainaudit.Event{
		ID: provider.NewID("audit"), EventType: eventType,
		ActorID: value.ActorID, ActorType: "user", EnvironmentID: environmentID,
		ResourceID: value.ResourceID, ConnectionID: value.ConnectionID,
		RuntimeID: value.RuntimeID, SessionID: value.ID, ExternalID: value.ExternalID,
		Outcome: outcome, Summary: summary, Metadata: map[string]any{"interactive": true},
		OccurredAt: time.Now().UTC(),
	})
}
