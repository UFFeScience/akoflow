package mcpcontrol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/api/httpserver"
	"github.com/UFFeScience/akoflow/internal/mcpcontrol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fixture struct {
	server    *httptest.Server
	manager   *mcpcontrol.Manager
	directory string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	server := httptest.NewUnstartedServer(nil)
	directory := t.TempDir()
	manager, err := mcpcontrol.New(mcpcontrol.Options{
		APIURL: "http://" + server.Listener.Addr().String() + "/akoflow-api", APIToken: "operator-secret", Directory: directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	api := httpserver.SecureAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/akoflow-api/mcp/") {
			manager.ControlHandler().ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"desktop-test-instance","name":"Desktop test"}`))
	}), httpserver.SecurityOptions{BearerToken: "operator-secret"})
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			manager.ServeHTTP(w, r)
			return
		}
		api.ServeHTTP(w, r)
	})
	server.Start()
	t.Cleanup(func() { server.Close(); manager.Close() })
	return &fixture{server: server, manager: manager, directory: directory}
}
func (f *fixture) request(t *testing.T, method, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, _ := http.NewRequest(method, f.server.URL+path, bytes.NewReader(data))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	value := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	return response.StatusCode, value
}
func (f *fixture) enable(t *testing.T) {
	t.Helper()
	status, _ := f.request(t, "PUT", "/akoflow-api/mcp/settings/", map[string]bool{"enabled": true}, "operator-secret")
	if status != 200 {
		t.Fatal("MCP did not enable")
	}
}
func (f *fixture) add(t *testing.T, permission string) (string, string) {
	t.Helper()
	status, value := f.request(t, "POST", "/akoflow-api/mcp/clients/", map[string]string{"name": "My app", "app": "codex", "permission": permission}, "operator-secret")
	if status != 201 {
		t.Fatalf("create status=%d", status)
	}
	id := value["id"].(string)
	if _, leaks := value["token"]; leaks {
		t.Fatal("registration returned a secret")
	}
	_, credential := f.request(t, "GET", "/akoflow-api/mcp/credentials/"+id+"/", nil, "operator-secret")
	return id, credential["token"].(string)
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	request := r.Clone(r.Context())
	request.Header = r.Header.Clone()
	request.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(request)
}
func TestManagedDesktopLifecycleAndProtocol(t *testing.T) {
	f := newFixture(t)
	status, _ := f.request(t, "GET", "/akoflow-api/mcp/status/", nil, "")
	if status != 401 {
		t.Fatal("management was not authenticated")
	}
	f.enable(t)
	id, token := f.add(t, "read")
	client := mcp.NewClient(&mcp.Implementation{Name: "Desktop SDK test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: f.server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearer{token}}, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	count := 0
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name == "factory_reset" || tool.Name == "create_workflow" {
			t.Fatal("read app received write tools")
		}
		count++
	}
	status, probe := f.request(t, "POST", "/akoflow-api/mcp/clients/"+id+"/probe/", nil, "operator-secret")
	if status != 200 || probe["healthy"] != true || int(probe["toolCount"].(float64)) != count {
		t.Fatalf("probe failed: %d %#v", status, probe)
	}
	status, state := f.request(t, "GET", "/akoflow-api/mcp/status/", nil, "operator-secret")
	encoded, _ := json.Marshal(state)
	if status != 200 || strings.Contains(string(encoded), token) {
		t.Fatal("status leaked a credential")
	}
	if !strings.Contains(string(encoded), "lastVerifiedAt") {
		t.Fatal("verification was not recorded")
	}
	mode, err := os.Stat(filepath.Join(f.directory, "credentials", id))
	if err != nil || mode.Mode().Perm() != 0600 {
		t.Fatal("credential file is not private")
	}
	status, _ = f.request(t, "DELETE", "/akoflow-api/mcp/clients/"+id+"/", nil, "operator-secret")
	if status != 204 {
		t.Fatal("revoke failed")
	}
	status, _ = f.request(t, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}, token)
	if status != 401 {
		t.Fatal("revoked credential still works")
	}
}
func TestPauseRotateAndPersistentConnections(t *testing.T) {
	f := newFixture(t)
	f.enable(t)
	id, oldToken := f.add(t, "admin")
	f.request(t, "PUT", "/akoflow-api/mcp/settings/", map[string]bool{"enabled": false}, "operator-secret")
	status, _ := f.request(t, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}, oldToken)
	if status != 503 {
		t.Fatal("disabled service accepted a client")
	}
	f.enable(t)
	status, _ = f.request(t, "POST", "/akoflow-api/mcp/clients/"+id+"/rotate/", nil, "operator-secret")
	if status != 200 {
		t.Fatal("rotation failed")
	}
	status, _ = f.request(t, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}, oldToken)
	if status != 401 {
		t.Fatal("old credential survived rotation")
	}
	_, credential := f.request(t, "GET", "/akoflow-api/mcp/credentials/"+id+"/", nil, "operator-secret")
	token := credential["token"].(string)
	if token == oldToken {
		t.Fatal("rotation reused the old token")
	}
	f.manager.Close()
	loaded, err := mcpcontrol.New(mcpcontrol.Options{APIURL: f.server.URL + "/akoflow-api", APIToken: "operator-secret", Directory: f.directory})
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	recorder := httptest.NewRecorder()
	loaded.ControlHandler().ServeHTTP(recorder, httptest.NewRequest("GET", "/akoflow-api/mcp/status/", nil))
	var state map[string]any
	json.Unmarshal(recorder.Body.Bytes(), &state)
	if state["enabled"] != true || len(state["clients"].([]any)) != 1 {
		t.Fatal("connections did not survive restart")
	}
	raw, err := os.ReadFile(filepath.Join(f.directory, "desktop-settings.json"))
	if err != nil || strings.Contains(string(raw), token) {
		t.Fatal("settings contain a secret")
	}
}
func TestConnectionValidationAndSecretEndpointProtection(t *testing.T) {
	f := newFixture(t)
	status, _ := f.request(t, "POST", "/akoflow-api/mcp/clients/", map[string]string{"name": "My app", "app": "codex", "permission": "read"}, "operator-secret")
	if status != 409 {
		t.Fatal("app created while MCP disabled")
	}
	f.enable(t)
	for _, value := range []map[string]string{
		{"name": "", "app": "codex", "permission": "read"},
		{"name": "App", "app": "unsupported", "permission": "read"},
		{"name": "App", "app": "codex", "permission": "root"},
	} {
		status, _ = f.request(t, "POST", "/akoflow-api/mcp/clients/", value, "operator-secret")
		if status != 422 {
			t.Fatalf("invalid app accepted: %d", status)
		}
	}
	id, token := f.add(t, "read")
	status, _ = f.request(t, "GET", "/akoflow-api/mcp/credentials/"+id+"/", nil, token)
	if status != 401 {
		t.Fatal("MCP credential granted operator access")
	}
}

func TestFactoryResetRevokesInstallationConnections(t *testing.T) {
	f := newFixture(t)
	f.enable(t)
	_, token := f.add(t, "admin")
	f.manager.Close()
	if err := mcpcontrol.ScheduleReset(f.directory); err != nil {
		t.Fatal(err)
	}
	restored, err := mcpcontrol.New(mcpcontrol.Options{
		APIURL: f.server.URL + "/akoflow-api", APIToken: "operator-secret", Directory: f.directory, InitiallyEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	status := httptest.NewRecorder()
	restored.ControlHandler().ServeHTTP(status, httptest.NewRequest("GET", "/akoflow-api/mcp/status/", nil))
	var state map[string]any
	if err := json.Unmarshal(status.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state["enabled"] != false || len(state["clients"].([]any)) != 0 {
		t.Fatal("factory reset retained MCP access")
	}
	for _, name := range []string{"credentials", "sessions", ".factory-reset-pending"} {
		if _, err := os.Stat(filepath.Join(f.directory, name)); !os.IsNotExist(err) {
			t.Fatalf("reset retained %s", name)
		}
	}
	request := httptest.NewRequest("PUT", "/akoflow-api/mcp/settings/", strings.NewReader(`{"enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	restored.ControlHandler().ServeHTTP(httptest.NewRecorder(), request)
	request = httptest.NewRequest("POST", "/mcp", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	restored.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatal("old client credential survived factory reset")
	}
}
