package transfer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
	"golang.org/x/crypto/ssh"
)

func TestConcurrentDirectCredentialRevocationPreservesUnrelatedKeys(t *testing.T) {
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("worker credential integration requires Linux flock; run this test in the worker-compatible container")
	}
	directory := t.TempDir()
	if strings.ContainsAny(directory, " '\"\n\t") {
		t.Fatal("fixture path must be shell-safe")
	}
	publicKey, _, err := ephemeralSSHKey()
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{"unrelated-user-key", "unrelated-transfer-token-suffix"}
	var revoked []string
	for i := 0; i < 8; i++ {
		line, err := scopedDirectAuthorizedKey(publicKey, fmt.Sprintf("transfer-%d", i), "cat > /tmp/private-archive", time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
		revoked = append(revoked, line)
	}
	keys := filepath.Join(directory, "authorized_keys")
	if err := os.WriteFile(keys, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for _, line := range revoked {
		wait.Add(1)
		go func(line string) {
			defer wait.Done()
			command := strings.ReplaceAll(directCredentialRemovalCommand(line), "~/.ssh", directory)
			if output, err := exec.Command("sh", "-c", command).CombinedOutput(); err != nil {
				t.Errorf("revoke: %v %s", err, output)
			}
		}(line)
	}
	wait.Wait()
	content, err := os.ReadFile(keys)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != strings.Join(lines[:2], "\n")+"\n" {
		t.Fatalf("unrelated keys lost or credentials retained: %s", content)
	}
	info, err := os.Stat(keys)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe final authorized_keys permissions")
	}
}

func TestShellQuotingRoundTripsThroughNestedPOSIXCommands(t *testing.T) {
	for _, value := range []string{"", "plain", "worker's file", "'\"$HOME; $(false)\nsecond line", "back\\slash"} {
		for _, nested := range []bool{false, true} {
			command := "printf '%s' " + shell(value)
			if nested {
				command = "sh -c " + shell(command)
			}
			output, err := exec.Command("sh", "-c", command).CombinedOutput()
			if err != nil || string(output) != value {
				t.Fatalf("nested=%v roundtrip failed: %v %q", nested, err, output)
			}
		}
	}
}

func TestEphemeralTransferKeyIsReadableByOpenSSH(t *testing.T) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Fatal("OpenSSH is required for transfer credential integration")
	}
	publicKey, privateKey, err := ephemeralSSHKey()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "identity")
	if err := os.WriteFile(path, privateKey, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(keygen, "-y", "-f", path).CombinedOutput()
	if err != nil {
		t.Fatalf("OpenSSH cannot load ephemeral identity: %v %s", err, output)
	}
	expected, _, _, _, expectedErr := ssh.ParseAuthorizedKey(publicKey)
	actual, _, _, _, actualErr := ssh.ParseAuthorizedKey(output)
	if expectedErr != nil || actualErr != nil || !bytes.Equal(expected.Marshal(), actual.Marshal()) {
		t.Fatal("OpenSSH identity does not match installed authorization")
	}
}

func TestDirectCredentialIsForcedAndExpiring(t *testing.T) {
	publicKey, _, err := ephemeralSSHKey()
	if err != nil {
		t.Fatal(err)
	}
	command := "cat > '/tmp/preparation/archive'"
	line, err := scopedDirectAuthorizedKey(publicKey, "transfer-token", command, time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	_, comment, options, rest, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil || len(rest) != 0 || comment != "transfer-token" {
		t.Fatalf("invalid authorized key syntax: %v", err)
	}
	if !reflect.DeepEqual(options, []string{"restrict", `command="cat > '/tmp/preparation/archive'"`, `expiry-time="20261008090000Z"`}) {
		t.Fatalf("authorization must not allow arbitrary commands: %#v", options)
	}
	for _, invalid := range []string{"", "cat > file\nother-command", "cat\x00"} {
		if _, err := scopedDirectAuthorizedKey(publicKey, "token", invalid, time.Now()); err == nil {
			t.Fatal("unsafe forced command accepted")
		}
	}
	remove := directCredentialRemovalCommand(line)
	if !strings.Contains(remove, "flock") || !strings.Contains(remove, "grep -F -x -v") || !strings.Contains(remove, "chmod 600") {
		t.Fatal("revocation must serialize exact-line removal and preserve permissions")
	}
}

func installFakeSSH(t *testing.T) {
	t.Helper()
	directory := t.TempDir()
	script := `#!/bin/sh
command=""
for argument in "$@"; do command="$argument"; done
case "$command" in
  *"sha256sum --"*checksum-error*) echo 'checksum unavailable' >&2; exit 2 ;;
  *"sha256sum --"*) printf '%064d  artifact\n' 0; exit 0 ;;
  *"cat >"*upload-rejected*) echo 'remote filesystem quota exceeded' >&2; exit 23 ;;
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
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", t.TempDir())
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
	expectedArguments := []string{
		"-i /keys/id", "BatchMode=yes", "-i /query/key", "UserKnownHostsFile=/known",
		"StrictHostKeyChecking=yes", "-p 2222", "ProxyCommand=", "HostKeyAlias=alias",
		"-A", "ControlMaster=auto", "ControlPath=", "ControlPersist=180",
	}
	for _, expected := range expectedArguments {
		if !strings.Contains(joined, expected) {
			t.Fatalf("args=%q missing %q", joined, expected)
		}
	}
	if shell("a'b") != `'a'"'"'b'` || !reflect.DeepEqual(sshArgs(domain.TransferEndpoint{}), []string{"-o", "BatchMode=yes", "-o", "ConnectionAttempts=1", "-o", "ConnectTimeout=10"}) {
		t.Fatalf("SSH helper output mismatch: shell=%q args=%#v", shell("a'b"), sshArgs(domain.TransferEndpoint{}))
	}
}

func TestTransferSharesCanonicalConnectionSession(t *testing.T) {
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", t.TempDir())
	endpoint := domain.TransferEndpoint{
		ConnectionID: "cloud-connection-instance-a",
		URI:          "ssh://worker@example.test/workspace?port=22&identityFile=/keys/id&knownHostsFile=/known&hostKeyAlias=instance-a",
	}
	_, expected, err := provider.SSHMultiplexArguments(provider.SSHSessionKey{
		ConnectionID: endpoint.ConnectionID, Username: "worker", Host: "example.test", Port: 22,
		IdentityFile: "/keys/id", KnownHostsFile: "/known", HostKeyAlias: "instance-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if actual := sshControlPath(sshArgs(endpoint)); actual != expected {
		t.Fatalf("transfer session %q differs from canonical connection session %q", actual, expected)
	}
	endpoint.ConnectionID = "cloud-connection-instance-b"
	if actual := sshControlPath(sshArgs(endpoint)); actual == expected {
		t.Fatal("distinct connections must retain distinct channel limits")
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

func TestSSHUploadPreservesEarlyExitDiagnostic(t *testing.T) {
	installFakeSSH(t)
	err := (RsyncSSH{}).Put(context.Background(), sshTestEndpoint(), "upload-rejected", bytes.NewReader(bytes.Repeat([]byte("x"), 1024*1024)), 0)
	if err == nil || !strings.Contains(err.Error(), "remote filesystem quota exceeded") || !strings.Contains(err.Error(), "exit status 23") {
		t.Fatalf("remote upload diagnostic lost: %v", err)
	}
}

func TestSSHRemoteChecksumMatchAndMismatch(t *testing.T) {
	installFakeSSH(t)
	connector := RsyncSSH{}
	for _, tc := range []struct {
		digest  string
		matches bool
	}{
		{"sha256:" + strings.Repeat("0", 64), true},
		{"sha256:" + strings.Repeat("1", 64), false},
	} {
		matches, err := connector.VerifyDigest(context.Background(), sshTestEndpoint(), "present", tc.digest)
		if err != nil || matches != tc.matches {
			t.Fatalf("matches=%v expected=%v err=%v", matches, tc.matches, err)
		}
	}
	if _, err := connector.VerifyDigest(context.Background(), sshTestEndpoint(), "checksum-error", strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "checksum unavailable") {
		t.Fatalf("checksum failure lost: %v", err)
	}
}

func TestTransferSSHConnectionIsBounded(t *testing.T) {
	args := strings.Join(sshArgs(sshTestEndpoint()), " ")
	for _, option := range []string{"BatchMode=yes", "ConnectionAttempts=1", "ConnectTimeout=10"} {
		if !strings.Contains(args, option) {
			t.Fatalf("missing %s in %s", option, args)
		}
	}
	command, release, err := queuedSSHCommand(context.Background(), domain.TransferEndpoint{}, []string{"host", "true"})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if command.WaitDelay <= 0 {
		t.Fatal("pipe cleanup is unbounded")
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

func TestDestinationUserHostPreservesUserForPrivateRoute(t *testing.T) {
	endpoint := sshTestEndpoint()
	if got := destinationUserHost(endpoint, "10.0.0.7"); got != "researcher@10.0.0.7" {
		t.Fatalf("host=%q", got)
	}
}
