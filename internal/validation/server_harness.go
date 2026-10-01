// Package validation provides a deterministic AkôFlow server harness used only
// by the example-validation test suite. The harness boots the real server
// binary as a subprocess on a loopback port, then exercises example fixtures
// against it via HTTP. This mirrors how operators deploy AkôFlow and avoids
// the complexity of re-wiring every production dependency in-process.
package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ServerHarness runs the real akoflow-server binary on a loopback port and
// exposes convenience methods for posting example YAML payloads.
type ServerHarness struct {
	t        *testing.T
	binary   string
	stateDir string
	cmd      *exec.Cmd
	baseURL  string
	apiURL   string
}

// StartServer compiles (once per test run) and starts an isolated AkôFlow
// server on a free loopback port. All state lives under a per-test temp dir.
func StartServer(t *testing.T) *ServerHarness {
	t.Helper()

	stateDir := t.TempDir()
	binary := ensureServerBinary(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	databasePath := filepath.Join(stateDir, "akoflow.sqlite")
	instanceRoot := filepath.Join(stateDir, "instances")
	artifactRoot := filepath.Join(stateDir, "artifacts")
	require.NoError(t, os.MkdirAll(instanceRoot, 0o700))
	require.NoError(t, os.MkdirAll(artifactRoot, 0o700))

	cmd := exec.Command(binary)
	cmd.Dir = stateDir
	cmd.Env = append(os.Environ(),
		"AKOFLOW_DATABASE_PATH="+databasePath,
		"AKOFLOW_INSTANCE_ARCHIVE_ROOT="+instanceRoot,
		"AKOFLOW_ARTIFACT_STORE_ROOT="+artifactRoot,
		"AKOFLOW_HTTP_ADDRESS=127.0.0.1:"+portOf(address),
		"AKOFLOW_CONSOLE_ENABLED=false",
		"AKOFLOW_API_TOKEN=",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Capture stderr so failures are debuggable, but keep stdout empty so
	// the test runner output is not drowned by server logs.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())

	base := "http://127.0.0.1:" + portOf(address)
	h := &ServerHarness{
		t:        t,
		binary:   binary,
		stateDir: stateDir,
		cmd:      cmd,
		baseURL:  base,
		apiURL:   base + "/akoflow-api",
	}

	if err := h.waitReady(15 * time.Second); err != nil {
		_ = h.kill()
		t.Fatalf("server did not become ready: %v\nstderr:\n%s", err, stderr.String())
	}
	t.Cleanup(h.Stop)
	return h
}

// Binary returns the path to the compiled server binary used by the harness.
func (h *ServerHarness) Binary() string { return h.binary }

// API returns the akoflow-api base URL (with mount prefix).
func (h *ServerHarness) API() string { return h.apiURL }

// Base returns the bare loopback URL.
func (h *ServerHarness) Base() string { return h.baseURL }

// StateDir returns the per-test state directory used for database, artifacts,
// and instances.
func (h *ServerHarness) StateDir() string { return h.stateDir }

// PostYAML posts a YAML payload to the supplied endpoint with the
// application/yaml content type. The endpoint is resolved relative to the API
// base. It returns the status code and decoded body.
func (h *ServerHarness) PostYAML(endpoint string, payload []byte) (int, map[string]any, string) {
	h.t.Helper()
	target := strings.TrimRight(h.apiURL, "/") + "/" + strings.TrimLeft(endpoint, "/") + "/"
	request, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(payload))
	require.NoError(h.t, err)
	request.Header.Set("Content-Type", "application/yaml")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		h.t.Fatalf("POST %s: %v", target, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(h.t, err)
	var decoded map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &decoded); err != nil {
			return response.StatusCode, nil, string(body)
		}
	}
	return response.StatusCode, decoded, string(body)
}

// GetJSON fetches an endpoint and decodes the JSON response.
func (h *ServerHarness) GetJSON(endpoint string) (int, map[string]any, string) {
	h.t.Helper()
	target := strings.TrimRight(h.apiURL, "/") + "/" + strings.TrimLeft(endpoint, "/") + "/"
	request, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(h.t, err)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		h.t.Fatalf("GET %s: %v", target, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(h.t, err)
	var decoded map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &decoded); err != nil {
			return response.StatusCode, nil, string(body)
		}
	}
	return response.StatusCode, decoded, string(body)
}

// Stop terminates the server subprocess. Safe to call multiple times.
func (h *ServerHarness) Stop() {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return
	}
	_ = h.kill()
}

func (h *ServerHarness) kill() error {
	if h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(h.cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = h.cmd.Process.Signal(syscall.SIGTERM)
	}
	done := make(chan error, 1)
	go func() { done <- h.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = h.cmd.Process.Kill()
		<-done
	}
	h.cmd = nil
	return nil
}

func (h *ServerHarness) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	target := h.baseURL + "/"
	for time.Now().Before(deadline) {
		response, err := http.Get(target)
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server did not respond at %s within %s", target, timeout)
}

// ensureServerBinary compiles cmd/server into a cache directory and returns
// the path to the resulting binary. The build is cached by Go's standard
// mechanism; this helper just pins the output location so the harness can
// exec it directly.
func ensureServerBinary(t *testing.T) string {
	t.Helper()
	cacheDir := os.Getenv("AKOFLOW_VALIDATION_CACHE")
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "akoflow-validation")
	}
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	binary := filepath.Join(cacheDir, "akoflow-server")
	if _, err := os.Stat(binary); err == nil {
		if _, err := os.Stat(filepath.Join(cacheDir, ".stamp")); err == nil {
			return binary
		}
	}
	repoRoot, err := RepositoryRoot()
	require.NoError(t, err)
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/server")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build server binary: %v\n%s", err, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, ".stamp"), []byte(time.Now().UTC().Format(time.RFC3339)), 0o644))
	return binary
}

func portOf(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	_ = host
	return port
}

// Silence unused import warnings on platforms where context is unused.
var _ = context.Background
