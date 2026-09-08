package transfer

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func installFakeSSH(t *testing.T) {
	t.Helper()
	directory := t.TempDir()
	script := `#!/bin/sh
command=""
for argument in "$@"; do command="$argument"; done
case "$command" in
  *missing*) exit 1 ;;
  *transport-error*) echo 'remote unavailable' >&2; exit 2 ;;
  *"cat --"*|*"tail -c"*) printf payload; exit 0 ;;
  *) cat >/dev/null 2>/dev/null || true; exit 0 ;;
esac
`
	file := filepath.Join(directory, "ssh")
	if err := os.WriteFile(file, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func sshTestEndpoint() domain.TransferEndpoint {
	return domain.TransferEndpoint{URI: "ssh://researcher@example.test/scratch/project", Configuration: map[string]string{
		"identityFile": "/keys/id", "sshOptions": "-o BatchMode=yes",
	}}
}

func TestSSHTargetAndArguments(t *testing.T) {
	endpoint := sshTestEndpoint()
	host, target, err := sshTarget(endpoint, "runs/result.dat")
	if err != nil || host != "researcher@example.test" || target != "/scratch/project/runs/result.dat" {
		t.Fatalf("host=%q target=%q err=%v", host, target, err)
	}
	for _, value := range []domain.TransferEndpoint{{URI: "http://host/path"}, {URI: "ssh://host"}, {URI: "://bad"}} {
		if _, _, err := sshTarget(value, "file"); err == nil {
			t.Fatalf("invalid endpoint accepted: %+v", value)
		}
	}
	for _, name := range []string{"../outside", "/absolute"} {
		if _, _, err := sshTarget(endpoint, name); err == nil {
			t.Fatalf("escaping path accepted: %q", name)
		}
	}
	endpoint.URI += "?identityFile=/query/key&knownHostsFile=/known&port=2222&proxyCommand=ssh%20jump&hostKeyAlias=alias&forwardAgent=true"
	args := sshArgs(endpoint)
	joined := strings.Join(args, " ")
	for _, expected := range []string{"-i /keys/id", "BatchMode=yes", "-i /query/key", "UserKnownHostsFile=/known", "StrictHostKeyChecking=yes", "-p 2222", "ProxyCommand=", "HostKeyAlias=alias", "-A"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("args=%q missing %q", joined, expected)
		}
	}
	if shell("a'b") != `'a'\"'\"'b'` || !reflect.DeepEqual(sshArgs(domain.TransferEndpoint{}), []string{}) {
		t.Fatalf("SSH helper output mismatch: shell=%q args=%#v", shell("a'b"), sshArgs(domain.TransferEndpoint{}))
	}
}

func TestRsyncSSHCommandLifecycle(t *testing.T) {
	installFakeSSH(t)
	connector := RsyncSSH{BufferSize: func(context.Context) int { return 1024 }}
	endpoint := sshTestEndpoint()
	if !connector.CanHandle(endpoint) || connector.CanHandle(domain.TransferEndpoint{URI: "file:///tmp"}) {
		t.Fatal("unexpected connector selection")
	}
	exists, err := connector.Exists(context.Background(), endpoint, "present")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	exists, err = connector.Exists(context.Background(), endpoint, "missing")
	if err != nil || exists {
		t.Fatalf("missing exists=%v err=%v", exists, err)
	}
	if _, err := connector.Exists(context.Background(), endpoint, "transport-error"); err == nil || !strings.Contains(err.Error(), "remote unavailable") {
		t.Fatalf("transport error=%v", err)
	}
	reader, err := connector.Open(context.Background(), endpoint, "present", 2)
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || string(content) != "payload" {
		t.Fatalf("content=%q read=%v close=%v", content, readErr, closeErr)
	}
	if err := connector.Put(context.Background(), endpoint, "partial", bytes.NewReader([]byte("stream")), 0); err != nil {
		t.Fatal(err)
	}
	if err := connector.Put(context.Background(), endpoint, "partial", bytes.NewReader([]byte("resume")), 6); err != nil {
		t.Fatal(err)
	}
	if err := connector.Commit(context.Background(), endpoint, "partial", "final"); err != nil {
		t.Fatal(err)
	}
}

func TestRsyncSSHRejectsUnsafePathsBeforeExecution(t *testing.T) {
	connector := RsyncSSH{}
	endpoint := sshTestEndpoint()
	if _, err := connector.Open(context.Background(), endpoint, "../outside", 0); err == nil {
		t.Fatal("unsafe open path accepted")
	}
	if err := connector.Put(context.Background(), endpoint, "../outside", strings.NewReader("x"), 0); err == nil {
		t.Fatal("unsafe put path accepted")
	}
	if err := connector.Commit(context.Background(), endpoint, "../outside", "final"); err == nil {
		t.Fatal("unsafe partial path accepted")
	}
	if err := connector.Commit(context.Background(), endpoint, "partial", "../outside"); err == nil {
		t.Fatal("unsafe final path accepted")
	}
}

func TestRsyncSSHRuntimeRoutesRequireConcreteAndRestrictedCredentials(t *testing.T) {
	installFakeSSH(t)
	connector := RsyncSSH{}
	source := sshTestEndpoint()
	source.CloudInstanceID = "instance-a"
	destination := sshTestEndpoint()
	destination.CloudInstanceID = "instance-a"
	if networkBytes, err := connector.TransferRoute(context.Background(), domain.TransferRuntimeLocal, source, destination, "source", "target.partial", 0); err != nil || networkBytes != 0 {
		t.Fatalf("runtime local bytes=%d err=%v", networkBytes, err)
	}
	destination.CloudInstanceID = "instance-b"
	destination.URI = "ssh://researcher@target.test/scratch/project"
	if networkBytes, err := connector.TransferRoute(context.Background(), domain.TransferDirectRuntime, source, destination, "source", "target.partial", 0); err != nil || networkBytes != -1 {
		t.Fatalf("automatic direct route bytes=%d err=%v", networkBytes, err)
	}
	destination.Configuration["directIdentityFile"] = "/run/akoflow/ephemeral-key"
	destination.Configuration["directKnownHostsFile"] = "/run/akoflow/known-hosts"
	if networkBytes, err := connector.TransferRoute(context.Background(), domain.TransferDirectRuntime, source, destination, "source", "target.partial", 3); err != nil || networkBytes != -1 {
		t.Fatalf("direct route bytes=%d err=%v", networkBytes, err)
	}
}
