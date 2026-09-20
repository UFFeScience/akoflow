package provider

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHSessionPathUsesAllEffectiveConnectionSettings(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", directory)
	base := SSHSessionKey{
		ConnectionID: "hpc", Username: "researcher", Host: "secret.example", Port: 22,
		IdentityFile: "/keys/private", ProxyCommand: "ssh bastion", KnownHostsFile: "/keys/known",
		HostKeyAlias: "cluster", ForwardAgent: true,
	}
	arguments, first, err := SSHMultiplexArguments(base)
	if err != nil {
		t.Fatal(err)
	}
	_, same, _ := SSHMultiplexArguments(base)
	changed := base
	changed.ProxyCommand = "ssh another-bastion"
	_, otherProxy, _ := SSHMultiplexArguments(changed)
	changed = base
	changed.IdentityFile = "/keys/other"
	_, otherIdentity, _ := SSHMultiplexArguments(changed)
	if first != same || first == otherProxy || first == otherIdentity {
		t.Fatalf("paths first=%q same=%q proxy=%q identity=%q", first, same, otherProxy, otherIdentity)
	}
	name := filepath.Base(first)
	if len(name) != 32 || strings.Contains(first, base.Username) || strings.Contains(first, base.Host) || strings.Contains(first, "private") {
		t.Fatalf("control path exposes connection data: %q", first)
	}
	joined := strings.Join(arguments, " ")
	for _, expected := range []string{"ControlMaster=auto", "ControlPath=" + first, "ControlPersist=180"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("arguments %q lack %q", joined, expected)
		}
	}
}

func TestSSHMultiplexingCanBeDisabled(t *testing.T) {
	t.Setenv("AKOFLOW_SSH_MULTIPLEXING_ENABLED", "false")
	arguments, path, err := SSHMultiplexArguments(SSHSessionKey{Host: "host"})
	if err != nil || len(arguments) != 0 || path != "" {
		t.Fatalf("arguments=%v path=%q err=%v", arguments, path, err)
	}
}
