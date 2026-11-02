package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/api/httpserver"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config/logger"
	"github.com/UFFeScience/akoflow/internal/mcpbridge"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPUsesRealDaemonWorkflowContracts(t *testing.T) {
	t.Setenv("AKOFLOW_DATABASE_PATH", filepath.Join(t.TempDir(), "akoflow.db"))
	settings := config.Settings{HTTPAddress: ":0", DefaultNamespace: "akoflow"}
	app, err := newApplication(context.Background(), settings, logger.NewStdLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	daemon := httptest.NewServer(httpserver.SecureAPI(httpserver.NewMux(app.api), httpserver.SecurityOptions{BearerToken: "integration-token"}))
	defer daemon.Close()
	bridge, err := mcpbridge.New(mcpbridge.Options{
		APIURL: daemon.URL + "/akoflow-api", APIToken: "integration-token", Permissions: mcpbridge.Write, StateDirectory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := bridge.Server().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "daemon-contract-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	workflow := map[string]any{
		"name": "MCP contract workflow",
		"spec": map[string]any{
			"namespace": "science", "image": "alpine:3.20",
			"activities": []any{map[string]any{
				"name": "prepare", "command": map[string]any{"entrypoint": "sh", "arguments": []string{"-c", "echo ready"}},
			}},
		},
	}
	created, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_workflow", Arguments: map[string]any{"requestId": "contract-create", "body": workflow},
	})
	if err != nil || created.IsError {
		t.Fatalf("create failed: %v %#v", err, created)
	}
	// Compare a fresh MCP read to the actual authenticated API projection.
	request, _ := http.NewRequest("GET", daemon.URL+"/akoflow-api/workflow-definitions/mcp-contract-workflow/", nil)
	request.Header.Set("Authorization", "Bearer integration-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 201 && response.StatusCode != 200 {
		t.Fatalf("API read status %d", response.StatusCode)
	}
	var direct any
	if err = json.NewDecoder(response.Body).Decode(&direct); err != nil {
		t.Fatal(err)
	}
	read, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_workflow", Arguments: map[string]any{"path": map[string]any{"workflowId": "mcp-contract-workflow"}},
	})
	if err != nil || read.IsError {
		t.Fatalf("read failed: %v %#v", err, read)
	}
	raw, _ := json.Marshal(read.StructuredContent)
	var wrapped map[string]any
	if err = json.Unmarshal(raw, &wrapped); err != nil {
		t.Fatal(err)
	}
	expected, _ := json.Marshal(direct)
	actual, _ := json.Marshal(wrapped["data"])
	if string(expected) != string(actual) {
		t.Fatal("MCP projection diverged from daemon API")
	}
}
