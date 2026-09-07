package transfer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// KubernetesExec streams files through a short-lived pod mounted on a PVC.
// URI format: kubernetes:///absolute/root?namespace=default&claim=my-pvc.
// Cluster credentials stay in endpoint Configuration and are never placed in
// the URI persisted in transfer plans.
type KubernetesExec struct {
	BufferSize BufferSizeProvider
	mu         sync.Mutex
	sessions   map[string]kubernetesTransferSession
}

type kubernetesTransferSession struct {
	pod     string
	cleanup func()
}

func (*KubernetesExec) CanHandle(endpoint domain.TransferEndpoint) bool {
	u, err := url.Parse(endpoint.URI)
	return err == nil && u.Scheme == "kubernetes"
}

func kubernetesSessionKey(endpoint domain.TransferEndpoint) string {
	return endpoint.URI + "\x00" + endpoint.Configuration["transferSessionId"]
}

func (connector *KubernetesExec) BeginTransferSession(ctx context.Context, endpoint domain.TransferEndpoint) error {
	if endpoint.Configuration["transferSessionId"] == "" {
		return fmt.Errorf("Kubernetes transfer session requires an id")
	}
	target, _, err := kubernetesTarget(endpoint, "")
	if err != nil {
		return err
	}
	key := kubernetesSessionKey(endpoint)
	connector.mu.Lock()
	if connector.sessions != nil {
		if _, exists := connector.sessions[key]; exists {
			connector.mu.Unlock()
			return nil
		}
	}
	connector.mu.Unlock()
	pod, cleanup, err := target.pod(ctx)
	if err != nil {
		return err
	}
	connector.mu.Lock()
	if connector.sessions == nil {
		connector.sessions = map[string]kubernetesTransferSession{}
	}
	if existing, exists := connector.sessions[key]; exists {
		connector.mu.Unlock()
		cleanup()
		_ = existing
		return nil
	}
	connector.sessions[key] = kubernetesTransferSession{pod: pod, cleanup: cleanup}
	connector.mu.Unlock()
	return nil
}

func (connector *KubernetesExec) EndTransferSession(_ context.Context, endpoint domain.TransferEndpoint) error {
	key := kubernetesSessionKey(endpoint)
	connector.mu.Lock()
	session, exists := connector.sessions[key]
	delete(connector.sessions, key)
	connector.mu.Unlock()
	if exists && session.cleanup != nil {
		session.cleanup()
	}
	return nil
}

func (connector *KubernetesExec) pod(ctx context.Context, target kubernetesTransferTarget, endpoint domain.TransferEndpoint) (string, func(), error) {
	key := kubernetesSessionKey(endpoint)
	if endpoint.Configuration["transferSessionId"] != "" {
		connector.mu.Lock()
		session, exists := connector.sessions[key]
		connector.mu.Unlock()
		if exists {
			return session.pod, func() {}, nil
		}
	}
	return target.pod(ctx)
}

type kubernetesTransferTarget struct {
	root, namespace, claim, server, token, caFile string
	runID, activityID                             string
	insecure                                      bool
	createClaim                                   bool
	claimBytes                                    int64
}

func kubernetesTarget(endpoint domain.TransferEndpoint, name string) (kubernetesTransferTarget, string, error) {
	u, err := url.Parse(endpoint.URI)
	if err != nil || u.Scheme != "kubernetes" {
		return kubernetesTransferTarget{}, "", fmt.Errorf("invalid Kubernetes transfer endpoint")
	}
	target := kubernetesTransferTarget{
		root: u.Path, namespace: u.Query().Get("namespace"), claim: u.Query().Get("claim"),
		server: endpoint.Configuration["server"], token: endpoint.Configuration["token"],
		caFile: endpoint.Configuration["caFile"], insecure: endpoint.Configuration["insecureSkipTLSVerify"] == "true",
		createClaim: u.Query().Get("createClaim") == "true",
		runID:       u.Query().Get("runId"), activityID: u.Query().Get("activityId"),
	}
	target.claimBytes, _ = strconv.ParseInt(u.Query().Get("claimBytes"), 10, 64)
	if target.namespace == "" {
		target.namespace = "default"
	}
	if target.token == "" && endpoint.Configuration["tokenFile"] != "" {
		contents, readErr := os.ReadFile(endpoint.Configuration["tokenFile"])
		if readErr != nil {
			return target, "", fmt.Errorf("read Kubernetes transfer credential: %w", readErr)
		}
		target.token = strings.TrimSpace(string(contents))
	}
	if target.root == "" || !filepath.IsAbs(target.root) || target.claim == "" || target.server == "" || target.token == "" {
		return target, "", fmt.Errorf("Kubernetes transfer endpoint requires absolute root, claim, server and credential")
	}
	key := filepath.Clean(filepath.FromSlash(name))
	if name != "" && (filepath.IsAbs(key) || key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator))) {
		return target, "", fmt.Errorf("Kubernetes transfer path escapes endpoint")
	}
	full := filepath.Join(target.root, key)
	if full != target.root && !strings.HasPrefix(full, target.root+string(filepath.Separator)) {
		return target, "", fmt.Errorf("Kubernetes transfer path escapes endpoint")
	}
	return target, full, nil
}

func (target kubernetesTransferTarget) args() []string {
	args := []string{"--server=" + target.server, "--token=" + target.token, "--namespace=" + target.namespace}
	if target.insecure {
		args = append(args, "--insecure-skip-tls-verify=true")
	} else if target.caFile != "" {
		args = append(args, "--certificate-authority="+target.caFile)
	}
	return args
}

func (target kubernetesTransferTarget) pod(ctx context.Context) (string, func(), error) {
	if err := target.ensureClaim(ctx); err != nil {
		return "", nil, err
	}
	name := fmt.Sprintf("akoflow-transfer-%d", time.Now().UnixNano())
	spec := map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": name, "labels": map[string]string{
			"app.kubernetes.io/managed-by": "akoflow", "akoflow.io/purpose": "workspace-transfer",
		}},
		"spec": map[string]any{
			"restartPolicy": "Never",
			"containers": []map[string]any{{"name": "transfer", "image": "busybox:1.36",
				"command":      []string{"/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"},
				"volumeMounts": []map[string]any{{"name": "workspace", "mountPath": target.root}}}},
			"volumes": []map[string]any{{"name": "workspace", "persistentVolumeClaim": map[string]any{"claimName": target.claim}}},
		},
	}
	if target.runID != "" && target.activityID != "" {
		spec["metadata"].(map[string]any)["annotations"] = map[string]string{
			"akoflow.io/run-id": target.runID, "akoflow.io/activity-id": target.activityID,
		}
	}
	body, _ := json.Marshal(spec)
	args := append(target.args(), "create", "-f", "-")
	command := exec.CommandContext(ctx, "kubectl", args...)
	command.Stdin = bytes.NewReader(body)
	if output, err := command.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("create Kubernetes transfer pod: %w: %s", err, strings.TrimSpace(string(output)))
	}
	cleanup := func() {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = exec.CommandContext(deleteCtx, "kubectl", append(target.args(), "delete", "pod", name, "--wait=false", "--ignore-not-found=true")...).Run()
	}
	waitArgs := append(target.args(), "wait", "--for=condition=Ready", "pod/"+name, "--timeout=90s")
	if output, err := exec.CommandContext(ctx, "kubectl", waitArgs...).CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("wait for Kubernetes transfer pod: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return name, cleanup, nil
}

func (target kubernetesTransferTarget) ensureClaim(ctx context.Context) error {
	if !target.createClaim {
		return nil
	}
	storageBytes := target.claimBytes
	if storageBytes < 64<<20 {
		storageBytes = 64 << 20
	}
	spec := map[string]any{
		"apiVersion": "v1", "kind": "PersistentVolumeClaim",
		"metadata": map[string]any{"name": target.claim, "labels": map[string]string{
			"app.kubernetes.io/managed-by": "akoflow", "akoflow.io/purpose": "workspace-transfer",
		}},
		"spec": map[string]any{
			"accessModes": []string{"ReadWriteOnce"},
			"resources":   map[string]any{"requests": map[string]string{"storage": strconv.FormatInt(storageBytes, 10)}},
		},
	}
	if target.runID != "" && target.activityID != "" {
		spec["metadata"].(map[string]any)["annotations"] = map[string]string{
			"akoflow.io/run-id": target.runID, "akoflow.io/activity-id": target.activityID,
		}
	}
	body, _ := json.Marshal(spec)
	command := exec.CommandContext(ctx, "kubectl", append(target.args(), "create", "-f", "-")...)
	command.Stdin = bytes.NewReader(body)
	output, err := command.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "AlreadyExists") {
		return fmt.Errorf("create Kubernetes workspace claim: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (target kubernetesTransferTarget) exec(ctx context.Context, pod, script string) *exec.Cmd {
	return exec.CommandContext(ctx, "kubectl", append(target.args(), "exec", "-i", pod, "--", "/bin/sh", "-c", script)...)
}

func (connector *KubernetesExec) Exists(ctx context.Context, endpoint domain.TransferEndpoint, name string) (bool, error) {
	target, file, err := kubernetesTarget(endpoint, name)
	if err != nil {
		return false, err
	}
	pod, cleanup, err := connector.pod(ctx, target, endpoint)
	if err != nil {
		return false, err
	}
	defer cleanup()
	output, err := target.exec(ctx, pod, "test -f "+shell(file)).CombinedOutput()
	if err == nil {
		return true, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check Kubernetes transfer location: %w: %s", err, strings.TrimSpace(string(output)))
}

func (connector *KubernetesExec) Open(ctx context.Context, endpoint domain.TransferEndpoint, name string, offset int64) (io.ReadCloser, error) {
	target, file, err := kubernetesTarget(endpoint, name)
	if err != nil {
		return nil, err
	}
	pod, cleanup, err := connector.pod(ctx, target, endpoint)
	if err != nil {
		return nil, err
	}
	script := "cat -- " + shell(file)
	if offset > 0 {
		script = fmt.Sprintf("tail -c +%d -- %s", offset+1, shell(file))
	}
	command := target.exec(ctx, pod, script)
	output, err := command.StdoutPipe()
	if err != nil {
		cleanup()
		return nil, err
	}
	var stderr strings.Builder
	command.Stderr = &stderr
	if err = command.Start(); err != nil {
		cleanup()
		return nil, err
	}
	return readCloser{Reader: output, close: func() error {
		_ = output.Close()
		waitErr := command.Wait()
		cleanup()
		if waitErr != nil {
			return fmt.Errorf("read Kubernetes transfer stream: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		return nil
	}}, nil
}

func (connector *KubernetesExec) Put(ctx context.Context, endpoint domain.TransferEndpoint, name string, input io.Reader, offset int64) error {
	target, file, err := kubernetesTarget(endpoint, name)
	if err != nil {
		return err
	}
	pod, cleanup, err := connector.pod(ctx, target, endpoint)
	if err != nil {
		return err
	}
	defer cleanup()
	script := "mkdir -p -- " + shell(filepath.Dir(file)) + " && cat > " + shell(file)
	if offset > 0 {
		script = fmt.Sprintf("test $(wc -c < %s) -eq %d && cat >> %s", shell(file), offset, shell(file))
	}
	command := target.exec(ctx, pod, script)
	bufferSize := int64(0)
	if connector.BufferSize != nil {
		bufferSize = int64(connector.BufferSize(ctx))
	}
	bufferedInput := bufio.NewReaderSize(input, normalizeBufferSize(bufferSize))
	// Complete a potentially slow source handshake before opening the
	// Kubernetes stdin stream. Peek retains the byte in the fixed relay buffer.
	if _, peekErr := bufferedInput.Peek(1); peekErr != nil && peekErr != io.EOF {
		return fmt.Errorf("prime Kubernetes transfer source: %w", peekErr)
	}
	relay := newBoundedRelay(bufferedInput, normalizeBufferSize(bufferSize))
	defer relay.Close()
	command.Stdin = relay
	var stderr strings.Builder
	command.Stderr = &stderr
	runErr := command.Run()
	if runErr != nil {
		return fmt.Errorf("stream to Kubernetes staging: %w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (connector *KubernetesExec) Commit(ctx context.Context, endpoint domain.TransferEndpoint, partial, final string) error {
	target, source, err := kubernetesTarget(endpoint, partial)
	if err != nil {
		return err
	}
	_, destination, err := kubernetesTarget(endpoint, final)
	if err != nil {
		return err
	}
	pod, cleanup, err := connector.pod(ctx, target, endpoint)
	if err != nil {
		return err
	}
	defer cleanup()
	output, err := target.exec(ctx, pod, "mkdir -p -- "+shell(filepath.Dir(destination))+" && mv -- "+shell(source)+" "+shell(destination)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("commit Kubernetes transfer: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
