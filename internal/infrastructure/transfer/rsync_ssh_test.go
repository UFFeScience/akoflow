package transfer

import (
	"net/url"
	"os"
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

func TestRsyncConfigAppliesProxyCredentialAndPort(t *testing.T) {
	query := url.Values{
		"identityFile":   {"/keys/personal"},
		"knownHostsFile": {"/keys/known_hosts"},
		"proxyCommand":   {"ssh gateway -W target:22"},
		"port":           {"2222"},
	}
	endpoint := domain.TransferEndpoint{URI: "ssh://user@target/data?" + query.Encode()}
	configuration, _, cleanup, err := rsyncSSHConfig(endpoint, "user@target")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	contents, err := os.ReadFile(configuration)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{"IdentityFile /keys/personal", "Port 2222", "UserKnownHostsFile /keys/known_hosts", "ProxyCommand ssh", "-i '/keys/personal'"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("rsync SSH config %q does not contain %q", text, expected)
		}
	}
}
