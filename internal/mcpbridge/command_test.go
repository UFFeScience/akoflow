package mcpbridge

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioProcessHelper(t *testing.T) {
	if os.Getenv("AKOFLOW_MCP_TEST_CHILD") != "1" {
		return
	}
	if err := RunCommand(context.Background(), nil, os.Stderr); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestStdioTransportAgainstDaemon(t *testing.T) {
	bridge := testBridge(t, Read, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer daemon-secret" {
			t.Error("stdio process lost daemon token")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"stdio-instance"}`))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStdioProcessHelper$")
	command.Env = append(os.Environ(),
		"AKOFLOW_MCP_TEST_CHILD=1", "AKOFLOW_API_URL="+bridge.options.APIURL,
		"AKOFLOW_API_TOKEN=daemon-secret", "AKOFLOW_MCP_STATE_DIRECTORY="+t.TempDir(),
		"AKOFLOW_MCP_PERMISSIONS=read",
	)
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := callTool(t, session, "get_instance", map[string]any{})
	if result.IsError {
		t.Fatal("stdio call failed")
	}
	if resultMap(t, result)["data"].(map[string]any)["id"] != "stdio-instance" {
		t.Fatal("stdio response corrupted")
	}
	if err = session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCommandRejectsInvalidConfiguration(t *testing.T) {
	for _, args := range [][]string{
		{"--transport", "unknown"}, {"--permissions", "root"}, {"--api-url", "file:///tmp/data"},
		{"--api-url", "http://host/akoflow-api?token=bad"}, {"extra-argument"},
	} {
		if err := RunCommand(context.Background(), args, io.Discard); err == nil {
			t.Fatalf("accepted invalid args %v", args)
		}
	}
}
