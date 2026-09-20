package provider

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type sshExecutorStub struct {
	name   string
	args   []string
	input  []byte
	output []byte
	err    error
	calls  int
	errors []error
}

func (s *sshExecutorStub) Run(_ context.Context, name string, args []string, input []byte) ([]byte, error) {
	s.calls++
	s.name, s.args, s.input = name, append([]string(nil), args...), append([]byte(nil), input...)
	if len(s.errors) >= s.calls {
		return s.output, s.errors[s.calls-1]
	}
	return s.output, s.err
}

func TestNewSSHCommandExecutorPreservesConnectionTransport(t *testing.T) {
	connection := domain.EnvironmentConnection{
		Endpoint: "plafrim", Username: "researcher", CredentialRef: "file:storage/credentials/ssh/personal",
		Configuration: map[string]any{
			"port": 2222, "proxyCommand": "ssh gateway -W plafrim:22",
			"hostKeyAlias": "plafrim", "forwardAgent": true,
		},
	}
	value := NewSSHCommandExecutor(OSCommandExecutor{}, connection)
	if value.Port != 2222 || value.IdentityFile != "storage/credentials/ssh/personal" {
		t.Fatalf("credential transport was not preserved: %#v", value)
	}
	if value.ProxyCommand == "" || value.HostKeyAlias != "plafrim" || !value.ForwardAgent {
		t.Fatalf("SSH connection options were not preserved: %#v", value)
	}
	if !strings.HasSuffix(value.KnownHostsFile, "storage/credentials/ssh/known_hosts") {
		t.Fatalf("default known_hosts was not applied: %q", value.KnownHostsFile)
	}
}

func TestProxyCommandUsesManagedCredential(t *testing.T) {
	value := ProxyCommandWithKnownHosts("ssh gateway -W target:22", "/keys/known_hosts", "/keys/personal")
	for _, expected := range []string{"UserKnownHostsFile='/keys/known_hosts'", "StrictHostKeyChecking=accept-new", "-i '/keys/personal'", "gateway -W target:22"} {
		if !strings.Contains(value, expected) {
			t.Fatalf("proxy command %q does not contain %q", value, expected)
		}
	}
}

func TestSSHCommandExecutorBuildsSafeTransportCommand(t *testing.T) {
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", t.TempDir())
	stub := &sshExecutorStub{output: []byte("ok")}
	knownHosts := filepath.Join(t.TempDir(), "ssh", "known_hosts")
	executor := SSHCommandExecutor{Executor: stub, Endpoint: "host", Username: "user", Port: 2222, IdentityFile: "/keys/id", KnownHostsFile: knownHosts, ProxyCommand: "ssh gateway -W host:22", HostKeyAlias: "alias", ForwardAgent: true}
	output, err := executor.Run(context.Background(), "sh", []string{"-c", "echo 'hello world'"}, []byte("input"))
	if err != nil || string(output) != "ok" {
		t.Fatalf("Run() = %q, %v", output, err)
	}
	joined := strings.Join(stub.args, " ")
	expectedArguments := []string{
		"BatchMode=yes", "UserKnownHostsFile=" + knownHosts, "-p 2222", "-i /keys/id",
		"ProxyCommand=ssh", "HostKeyAlias=alias", "-A", "ControlMaster=auto",
		"ControlPath=", "ControlPersist=180", "user@host", `'sh' '-c' 'echo '`,
	}
	for _, expected := range expectedArguments {
		if !strings.Contains(joined, expected) {
			t.Fatalf("args %q lack %q", joined, expected)
		}
	}
	if string(stub.input) != "input" || stub.name != "ssh" {
		t.Fatalf("invocation = %s %q", stub.name, stub.input)
	}
}

func TestSSHCommandExecutorRetriesOneBrokenControlSocket(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", directory)
	stub := &sshExecutorStub{output: []byte("Control socket connect: Connection refused"), errors: []error{fmt.Errorf("mux_client_request_session: master is dead"), nil}}
	executor := SSHCommandExecutor{Executor: stub, ConnectionID: "hpc", Endpoint: "host", Username: "user"}
	if _, err := executor.Run(context.Background(), "true", nil, nil); err != nil || stub.calls != 2 {
		t.Fatalf("calls=%d err=%v", stub.calls, err)
	}
}

func TestSSHCommandExecutorValidatesConfigurationAndPropagatesErrors(t *testing.T) {
	if _, err := (SSHCommandExecutor{}).Run(context.Background(), "true", nil, nil); err == nil {
		t.Fatal("expected executor error")
	}
	if _, err := (SSHCommandExecutor{Executor: &sshExecutorStub{}}).Run(context.Background(), "true", nil, nil); err == nil {
		t.Fatal("expected endpoint error")
	}
	stub := &sshExecutorStub{err: fmt.Errorf("ssh failed")}
	if _, err := (SSHCommandExecutor{Executor: stub, Endpoint: "user@host"}).Run(context.Background(), "true", nil, nil); err == nil {
		t.Fatal("expected delegated error")
	}
	if shellQuote("") != "''" || shellQuote("it's") == "'it's'" {
		t.Fatalf("shell quoting mismatch: %s", shellQuote("it's"))
	}
	if ProxyCommandWithKnownHosts("nc gateway", "/known", "") != "nc gateway" {
		t.Fatal("non-ssh proxy changed")
	}
	if connectionInt(map[string]any{"port": "2022"}, "port") != 2022 || connectionInt(map[string]any{"port": true}, "port") != 0 {
		t.Fatal("connectionInt mismatch")
	}
}

func TestShellQuotePreservesSingleQuotesThroughRemoteShell(t *testing.T) {
	input := `with open(full,'rb') as stream: print('ok')`
	output, err := exec.Command("sh", "-c", "printf %s "+shellQuote(input)).Output()
	if err != nil || string(output) != input {
		t.Fatalf("quoted command changed argument: %q, %v", output, err)
	}
}
