package ansible

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

func TestRunnerConfigureRequiresAddressBeforeTouchingWorkspace(t *testing.T) {
	err := (Runner{Root: t.TempDir()}).Configure(context.Background(), ports.MachineConfigurationSpec{
		CredentialRef: "file:/missing/key",
	})
	if err == nil || err.Error() != "machine configuration requires a public address" {
		t.Fatalf("expected address validation error, got %v", err)
	}
}

func TestRunnerValidateSucceedsWithoutChecks(t *testing.T) {
	root := t.TempDir()
	key := filepath.Join(root, "id_ed25519")
	if err := os.WriteFile(key, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (Runner{Root: root}).Validate(context.Background(), ports.MachineConfigurationSpec{
		InstanceID:    "instance-1",
		CredentialRef: "file:" + key,
	}); err != nil {
		t.Fatalf("expected validation with no checks to succeed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "instance-1", "provision.log")); err != nil {
		t.Fatalf("expected validation log: %v", err)
	}
}

func TestWaitForSSHHonorsContextCancellation(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "provision-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err = waitForSSH(ctx, ports.MachineConfigurationSpec{Address: "192.0.2.1", SSHUser: "akoflow"}, "/missing/key", logFile)
	if err == nil || err != context.Canceled {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancellation should not wait for the SSH retry timer")
	}
	if _, err := logFile.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(logFile)
	if err != nil || len(content) == 0 {
		t.Fatalf("expected retry details in provision log, got %q (%v)", content, err)
	}
}

func TestRunPlaybookUsesInjectedExecutorAndLogsOutput(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "ansible")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	logFile, err := os.CreateTemp(root, "provision-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	runner := Runner{Binary: "fake-ansible", Execute: func(_ context.Context, _ string, _ []string, commandAndArguments ...string) ([]byte, error) {
		if commandAndArguments[0] != "fake-ansible" || commandAndArguments[1] != "-i" {
			t.Fatalf("unexpected command: %#v", commandAndArguments)
		}
		return []byte("TASK [Gathering Facts] ok"), nil
	}}
	if err := runner.runPlaybook(context.Background(), workspace, ports.MachineConfigurationSpec{Address: "192.0.2.10", SSHUser: "akoflow"}, "/tmp/key", "/tmp/vars", filepath.Join(workspace, "playbook.yaml"), logFile); err != nil {
		t.Fatal(err)
	}
	if _, err := logFile.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(logFile)
	if !strings.Contains(string(content), "TASK [Gathering Facts] ok") || !strings.Contains(string(content), "playbook completed") {
		t.Fatalf("expected command output and phase log, got %q", content)
	}
}

func TestRunPlaybookReturnsCommandOutputOnFailure(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "provision-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	runner := Runner{Execute: func(context.Context, string, []string, ...string) ([]byte, error) {
		return []byte("playbook syntax error"), errors.New("exit status 2")
	}}
	err = runner.runPlaybook(context.Background(), t.TempDir(), ports.MachineConfigurationSpec{Address: "192.0.2.10", SSHUser: "akoflow"}, "/tmp/key", "/tmp/vars", "/tmp/playbook.yaml", logFile)
	if err == nil || !strings.Contains(err.Error(), "playbook syntax error") {
		t.Fatalf("expected command output in error, got %v", err)
	}
}

func TestPrivateKeyPathRejectsExternalSchemes(t *testing.T) {
	if _, err := privateKeyPath("vault:key"); err == nil {
		t.Fatal("expected unsupported credential reference")
	}
}

func TestPrivateKeyPathReturnsAbsolutePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, err := privateKeyPath("file:" + path)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("expected absolute path, got %q", resolved)
	}
}
