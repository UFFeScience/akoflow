package transfer

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func fakeKubectl(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	script := filepath.Join(directory, "kubectl")
	payload := `#!/bin/sh
printf '%s\n' "$*" >> "$AKOFLOW_TEST_KUBECTL_LOG"
case " $* " in
  *" exec "*)
    command="${@: -1}"
    case "$command" in
      *"cat --"*|*"tail -c"*) printf 'remote-data' ;;
      *) cat >/dev/null || true ;;
    esac
    ;;
esac
exit 0
`
	// POSIX sh on the development image does not support ${@: -1}; the command
	// text is only needed to distinguish reads, so inspect the full argument list.
	payload = strings.ReplaceAll(payload, `command="${@: -1}"`, `command="$*"`)
	if err := os.WriteFile(script, []byte(payload), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	log := filepath.Join(directory, "kubectl.log")
	t.Setenv("AKOFLOW_TEST_KUBECTL_LOG", log)
	return log
}

func TestKubernetesTargetResolvesCredentialAndPath(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte(" secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	endpoint := domain.TransferEndpoint{
		URI:           "kubernetes:///workspace?namespace=science&claim=data&createClaim=true&claimBytes=123&runId=run&activityId=a&nodeName=worker-a",
		Configuration: map[string]string{"server": "https://cluster", "tokenFile": tokenFile, "caFile": "/ca.pem"},
	}
	target, file, err := kubernetesTarget(endpoint, "nested/result.txt")
	if err != nil {
		t.Fatal(err)
	}
	if file != "/workspace/nested/result.txt" || target.token != "secret" || !target.createClaim || target.claimBytes != 123 || target.runID != "run" || target.activityID != "a" || target.nodeName != "worker-a" {
		t.Fatalf("target=%+v file=%q", target, file)
	}
	args := strings.Join(target.args(), " ")
	if !strings.Contains(args, "--certificate-authority=/ca.pem") || !strings.Contains(args, "--namespace=science") {
		t.Fatalf("args=%q", args)
	}
}

func TestKubernetesTargetValidatesEndpoint(t *testing.T) {
	valid := domain.TransferEndpoint{
		URI:           "kubernetes:///workspace?claim=data",
		Configuration: map[string]string{"server": "https://cluster", "token": "token", "insecureSkipTLSVerify": "true"},
	}
	target, _, err := kubernetesTarget(valid, "")
	if err != nil || target.namespace != "default" || !target.insecure {
		t.Fatalf("target=%+v err=%v", target, err)
	}
	if _, _, err := kubernetesTarget(valid, "../escape"); err == nil {
		t.Fatal("traversal must fail")
	}
	if _, _, err := kubernetesTarget(domain.TransferEndpoint{URI: "file:///tmp"}, "file"); err == nil {
		t.Fatal("wrong scheme must fail")
	}
	if _, _, err := kubernetesTarget(domain.TransferEndpoint{URI: "kubernetes:///relative"}, "file"); err == nil {
		t.Fatal("missing credentials must fail")
	}
}

func TestKubernetesExecLifecycleWithKubectl(t *testing.T) {
	fakeKubectl(t)
	endpoint := domain.TransferEndpoint{
		URI:           "kubernetes:///workspace?claim=data&createClaim=true&claimBytes=1&runId=run&activityId=a",
		Configuration: map[string]string{"server": "https://cluster", "token": "token"},
	}
	connector := KubernetesExec{BufferSize: func(context.Context) int { return 8192 }}
	exists, err := connector.Exists(context.Background(), endpoint, "result.txt")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	reader, err := connector.Open(context.Background(), endpoint, "result.txt", 2)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil || closeErr != nil || string(payload) != "remote-data" {
		t.Fatalf("payload=%q read=%v close=%v", payload, err, closeErr)
	}
	if err := connector.Put(context.Background(), endpoint, "result.partial", bytes.NewBufferString("payload"), 0); err != nil {
		t.Fatal(err)
	}
	if err := connector.Put(context.Background(), endpoint, "result.partial", bytes.NewBufferString("more"), 7); err != nil {
		t.Fatal(err)
	}
	if err := connector.Commit(context.Background(), endpoint, "result.partial", "result.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestKubernetesExecReusesOnePodForTransferSession(t *testing.T) {
	log := fakeKubectl(t)
	endpoint := domain.TransferEndpoint{
		URI: "kubernetes:///workspace?claim=data",
		Configuration: map[string]string{
			"server": "https://cluster", "token": "token", "transferSessionId": "transfer-1",
		},
	}
	connector := &KubernetesExec{}
	ctx := context.Background()
	if err := connector.BeginTransferSession(ctx, endpoint); err != nil {
		t.Fatal(err)
	}
	if _, err := connector.Exists(ctx, endpoint, "result.txt"); err != nil {
		t.Fatal(err)
	}
	reader, err := connector.Open(ctx, endpoint, "result.txt", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, reader)
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := connector.EndTransferSession(ctx, endpoint); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(contents), " wait --for=condition=Ready pod/"); count != 1 {
		t.Fatalf("created %d transfer pod sessions; log:\n%s", count, contents)
	}
}
