package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testBridge(t *testing.T, permission Permission, handler http.HandlerFunc) *Bridge {
	t.Helper()
	backend := httptest.NewServer(handler)
	t.Cleanup(backend.Close)
	bridge, err := New(Options{
		APIURL: backend.URL + "/akoflow-api", APIToken: "daemon-secret", Permissions: permission,
		StateDirectory: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bridge.Close() })
	return bridge
}

func testSession(t *testing.T, bridge *Bridge) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := bridge.server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return result
}

func resultMap(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func confirmed(t *testing.T, session *mcp.ClientSession, op Operation, args map[string]any) {
	t.Helper()
	result := callTool(t, session, "prepare_operation", map[string]any{"tool": op.Name, "arguments": args})
	if result.IsError {
		t.Fatalf("prepare %s: %#v", op.Name, result)
	}
	args["confirmation"] = resultMap(t, result)["confirmation"]
}

func TestEveryRegisteredOperationThroughMCP(t *testing.T) {
	var calls atomic.Int64
	bridge := testBridge(t, Admin, recordingDaemon(t, &calls))
	session := testSession(t, bridge)
	for index, op := range Catalog() {
		t.Run(op.Name, func(t *testing.T) {
			args := map[string]any{}
			params := map[string]string{}
			expected := op.Path
			for _, segment := range strings.Split(op.Path, "/") {
				if strings.HasPrefix(segment, "{") {
					name := strings.Trim(segment, "{}")
					params[name] = "fixture-id"
					expected = strings.ReplaceAll(expected, segment, "fixture-id")
				}
			}
			if len(params) > 0 {
				args["path"] = params
			}
			if op.Method != "GET" && op.Method != "DELETE" {
				args["body"] = map[string]any{}
			}
			if op.Name == "import_archive_instance" {
				id, _, err := bridge.transfers.save(strings.NewReader("zip fixture"))
				if err != nil {
					t.Fatal(err)
				}
				delete(args, "body")
				args["uploadId"] = id
			}
			if op.Name == "stream_console_session" {
				args["waitMillis"] = 1
			}
			if op.Permission() != Read {
				args["requestId"] = fmt.Sprintf("route-%d", index)
			}
			if op.Permission() == Admin {
				confirmed(t, session, op, args)
			}
			result := callTool(t, session, op.Name, args)
			if result.IsError {
				t.Fatalf("tool failed: %+v", result.Content)
			}
			if op.Name != "stream_console_session" {
				data := resultMap(t, result)["data"].(map[string]any)
				if data["method"] != op.Method || data["path"] != expected {
					t.Fatalf("wrong dispatch: %#v", data)
				}
			}
		})
	}
	if calls.Load() != int64(len(Catalog())+1) {
		t.Fatalf("calls=%d catalog=%d", calls.Load(), len(Catalog()))
	}
}

func TestPermissionsSchemaAndPathValidation(t *testing.T) {
	var calls atomic.Int64
	bridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.Write([]byte("[]")) })
	session := testSession(t, bridge)
	names := map[string]bool{}
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names[tool.Name] = true
	}
	if names["create_workflow"] || names["factory_reset"] || names["begin_upload"] {
		t.Fatal("read profile advertises mutations")
	}
	for _, args := range []map[string]any{
		{"path": map[string]any{"environmentId": "../escape"}},
		{"path": map[string]any{"environmentId": "a%2fb"}},
		{"path": map[string]any{"environmentId": "a/b"}},
		{"path": map[string]any{"environmentId": 42}},
		{"path": map[string]any{"environmentId": "ok"}, "unexpected": true},
	} {
		if !callTool(t, session, "get_environment", args).IsError {
			t.Fatal("unsafe input accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid requests reached the daemon")
	}
}

func TestDurableReceiptsAndConcurrentRetries(t *testing.T) {
	var calls atomic.Int64
	bridge := testBridge(t, Write, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"id":"queued-run"}`))
	})
	args := arguments{RequestID: "same-request", Body: json.RawMessage(`{}`)}
	raw, _ := json.Marshal(args)
	op := findOperation(t, "create_execution")
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() { defer group.Done(); bridge.call(context.Background(), op, raw) }()
	}
	group.Wait()
	if calls.Load() != 1 {
		t.Fatalf("dispatched %d times", calls.Load())
	}
	bridge.ledger.Close()
	ledger, err := newLedger(bridge.options.StateDirectory, bridge.options.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	bridge.ledger = ledger
	if result := bridge.call(context.Background(), op, raw); result.IsError {
		t.Fatalf("receipt not restored: %#v", result)
	}
	if calls.Load() != 1 {
		t.Fatal("restarted bridge repeated mutation")
	}
	args.Body = json.RawMessage(`{"run":{"id":"different"}}`)
	raw, _ = json.Marshal(args)
	if !bridge.call(context.Background(), op, raw).IsError {
		t.Fatal("conflicting requestId accepted")
	}
	if _, err = ledger.begin("pending", "hash"); err != nil {
		t.Fatal(err)
	}
	if _, err = ledger.begin("pending", "hash"); err == nil {
		t.Fatal("pending request was retried")
	}
}

func findOperation(t *testing.T, name string) Operation {
	t.Helper()
	for _, op := range Catalog() {
		if op.Name == name {
			return op
		}
	}
	t.Fatalf("missing operation %s", name)
	return Operation{}
}

func TestConfirmationBindsExactArguments(t *testing.T) {
	var calls atomic.Int64
	bridge := testBridge(t, Admin, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusNoContent) })
	session := testSession(t, bridge)
	op := findOperation(t, "factory_reset")
	args := map[string]any{"requestId": "reset-one", "body": map[string]any{}}
	if !callTool(t, session, op.Name, args).IsError {
		t.Fatal("missing confirmation accepted")
	}
	confirmed(t, session, op, args)
	args["body"] = map[string]any{"changed": true}
	if !callTool(t, session, op.Name, args).IsError {
		t.Fatal("confirmation accepted changed arguments")
	}
	args["body"] = map[string]any{}
	if callTool(t, session, op.Name, args).IsError {
		t.Fatal("confirmed request rejected")
	}
	if calls.Load() != 1 {
		t.Fatal("unexpected dispatch count")
	}
}

func TestRedactionErrorsRedirectsAndLimits(t *testing.T) {
	bridge := testBridge(t, Admin, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"token":"secret-output","credentialRef":"keep-ref","nested":{"privateKey":"hidden"},"error":"Bearer daemon-secret"}`))
	})
	session := testSession(t, bridge)
	result := callTool(t, session, "list_environments", map[string]any{})
	raw, _ := json.Marshal(result)
	if !result.IsError || strings.Contains(string(raw), "secret-output") || strings.Contains(string(raw), "hidden") || strings.Contains(string(raw), "daemon-secret") {
		t.Fatalf("credential leaked: %s", raw)
	}
	if !strings.Contains(string(raw), "keep-ref") {
		t.Fatal("credential reference removed")
	}
	var redirected atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Store(true) }))
	defer destination.Close()
	response := &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{destination.URL}}, Body: io.NopCloser(strings.NewReader(""))}
	// Test the bridge client with a real redirecting daemon.
	redirectBridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) })
	if !redirectBridge.call(context.Background(), findOperation(t, "list_environments"), json.RawMessage(`{}`)).IsError {
		t.Fatal("redirect accepted")
	}
	response.Body.Close()
	if redirected.Load() {
		t.Fatal("daemon credentials followed a redirect")
	}
}

func recordingDaemon(t *testing.T, calls *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer daemon-secret" {
			t.Error("missing daemon authentication")
		}
		if strings.HasSuffix(r.URL.Path, "/stream/") {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			conn.WriteMessage(websocket.BinaryMessage, []byte("terminal ready"))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/instances/import/") {
			if r.Header.Get("Content-Type") != "application/zip" {
				t.Error("ZIP media type lost")
			}
			data, _ := io.ReadAll(r.Body)
			if string(data) != "zip fixture" {
				t.Error("upload bytes changed")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/akoflow-api/connection-tests/" {
			json.NewEncoder(w).Encode(map[string]any{"healthy": true, "method": r.Method, "path": r.URL.Path})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"method": r.Method, "path": r.URL.Path})
	}
}

func TestConfirmationDistinguishesOrderSensitiveBodyAliases(t *testing.T) {
	first := arguments{RequestID: "bound", Body: json.RawMessage(`{"id":"first","ID":"second"}`)}
	second := arguments{RequestID: "bound", Body: json.RawMessage(`{"ID":"second","id":"first"}`)}
	a, err := operationDigest("save_instance", first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := operationDigest("save_instance", second)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("confirmation did not distinguish differently decoded API bodies")
	}
}
func TestAdminPreparationRequiresImmutableUpload(t *testing.T) {
	bridge := testBridge(t, Admin, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) })
	session := testSession(t, bridge)
	id, err := bridge.transfers.create()
	if err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"tool": "save_kubernetes_token", "arguments": map[string]any{
		"uploadId": id, "requestId": "prepare-upload",
	}}
	if !callTool(t, session, "prepare_operation", args).IsError {
		t.Fatal("unsealed upload could be approved")
	}
	if _, err = bridge.transfers.append(id, 0, []byte("{}"), true); err != nil {
		t.Fatal(err)
	}
	if callTool(t, session, "prepare_operation", args).IsError {
		t.Fatal("sealed upload could not be prepared")
	}
}
