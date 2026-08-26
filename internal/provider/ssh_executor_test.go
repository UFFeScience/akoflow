package provider

import (
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

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
