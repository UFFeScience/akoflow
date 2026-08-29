package transfer

import (
	"net/url"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestSSHArgsApplyProxyCredentialAndHostPolicy(t *testing.T) {
	query := url.Values{
		"identityFile":   {"/keys/personal"},
		"knownHostsFile": {"/keys/known_hosts"},
		"proxyCommand":   {"ssh gateway -W target:22"},
		"hostKeyAlias":   {"target"},
		"port":           {"2222"},
		"forwardAgent":   {"true"},
	}
	endpoint := domain.TransferEndpoint{URI: "ssh://user@target/data?" + query.Encode()}
	arguments := strings.Join(sshArgs(endpoint), " ")
	for _, expected := range []string{"-i /keys/personal", "UserKnownHostsFile=/keys/known_hosts", "-p 2222", "HostKeyAlias=target", "-A", "ProxyCommand=ssh", "-i '/keys/personal'"} {
		if !strings.Contains(arguments, expected) {
			t.Fatalf("SSH arguments %q do not contain %q", arguments, expected)
		}
	}
}

func TestSSHTargetRejectsPathTraversal(t *testing.T) {
	endpoint := domain.TransferEndpoint{URI: "ssh://user@target/data"}
	if _, _, err := sshTarget(endpoint, "../secret"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}
