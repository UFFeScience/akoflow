package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/creack/pty"
)

// TerminalRunner creates a short-lived, explicitly labeled pod then attaches
// kubectl exec to it. It works for Kind and remote Kubernetes alike; Docker is
// never required by the control-plane container.
type TerminalRunner struct{ Fallback ClientConfig }

var _ ports.InteractiveConsoleRunner = TerminalRunner{}

func (r TerminalRunner) StartInteractive(ctx context.Context, connection domain.EnvironmentConnection, resource domain.Resource) (ports.InteractiveTerminal, error) {
	endpoint := strings.TrimSpace(connection.Endpoint)
	if endpoint == "" {
		endpoint = r.Fallback.Endpoint
	}
	token, err := resolveCredential(connection.CredentialRef)
	if err != nil {
		return nil, err
	}
	if token == "" {
		token = r.Fallback.Token
	}
	if endpoint == "" || token == "" {
		return nil, fmt.Errorf("Kubernetes interactive terminal needs endpoint and credential")
	}
	namespace := configString(connection.Configuration, "namespace")
	if namespace == "" {
		namespace = "default"
	}
	insecure := configBool(connection.Configuration, "insecureSkipTlsVerify", r.Fallback.InsecureSkipTLSVerify)
	client, err := NewClient(ClientConfig{Endpoint: endpoint, Token: token, CAFile: configString(connection.Configuration, "caFile"), InsecureSkipTLSVerify: insecure})
	if err != nil {
		return nil, err
	}
	pod := fmt.Sprintf("akoflow-console-%d", time.Now().UnixNano())
	nodeName, _ := resource.Metadata["observedHostname"].(string)
	if nodeName == "" && !strings.Contains(resource.ProviderID, "://") {
		nodeName = resource.ProviderID
	}
	spec := map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": pod, "labels": map[string]string{"app.kubernetes.io/managed-by": "akoflow", "akoflow.io/purpose": "interactive-console"}}, "spec": map[string]any{"restartPolicy": "Never", "nodeName": resource.ProviderID, "containers": []map[string]any{{"name": "console", "image": "busybox:1.36", "command": []string{"/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"}, "stdin": true, "tty": true}}}}
	if nodeName == "" {
		delete(spec["spec"].(map[string]any), "nodeName")
	} else {
		spec["spec"].(map[string]any)["nodeName"] = nodeName
	}
	body, _ := json.Marshal(spec)
	if err := client.Create(ctx, namespace, "pods", body); err != nil {
		return nil, fmt.Errorf("create interactive pod: %w", err)
	}
	if err := waitForPod(ctx, client, namespace, pod); err != nil {
		_ = client.Delete(context.Background(), namespace, "pods", pod)
		return nil, err
	}
	args := []string{"--server=" + client.endpoint, "--token=" + client.token, "--namespace=" + namespace}
	if insecure {
		args = append(args, "--insecure-skip-tls-verify=true")
	}
	args = append(args, "exec", "-it", pod, "--", "/bin/sh", "-l")
	command := exec.Command("kubectl", args...)
	terminal, err := pty.Start(command)
	if err != nil {
		_ = client.Delete(context.Background(), namespace, "pods", pod)
		return nil, fmt.Errorf("start Kubernetes terminal: %w", err)
	}
	return &terminalHandle{file: terminal, command: command, cleanup: func() { _ = client.Delete(context.Background(), namespace, "pods", pod) }}, nil
}

func waitForPod(ctx context.Context, client *Client, namespace, pod string) error {
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		payload, err := client.Get(ctx, namespace, "pods", pod)
		if err == nil {
			var value struct{ Status struct{ Phase string } }
			_ = json.Unmarshal(payload, &value)
			if value.Status.Phase == "Running" {
				return nil
			}
			if value.Status.Phase == "Failed" || value.Status.Phase == "Succeeded" {
				return fmt.Errorf("interactive pod %q ended as %s", pod, value.Status.Phase)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("interactive pod %q did not become ready", pod)
		case <-tick.C:
		}
	}
}

type terminalHandle struct {
	file    *os.File
	command *exec.Cmd
	cleanup func()
	once    sync.Once
}

func (h *terminalHandle) Read(b []byte) (int, error)  { return h.file.Read(b) }
func (h *terminalHandle) Write(b []byte) (int, error) { return h.file.Write(b) }
func (h *terminalHandle) Resize(rows, columns uint16) error {
	return pty.Setsize(h.file, &pty.Winsize{Rows: rows, Cols: columns})
}
func (h *terminalHandle) Close() (err error) {
	h.once.Do(func() {
		_ = h.file.Close()
		if h.command.Process != nil {
			_ = h.command.Process.Kill()
		}
		err = h.command.Wait()
		h.cleanup()
	})
	return
}
