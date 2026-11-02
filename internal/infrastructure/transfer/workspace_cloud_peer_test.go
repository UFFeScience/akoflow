package transfer

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestCloudWorkspacePeerEligibilityFailsClosed(t *testing.T) {
	endpoint := func(id, address string) domain.TransferEndpoint {
		return domain.TransferEndpoint{
			CloudInstanceID: id, ConnectionID: "connection-" + id, EnvironmentID: "environment",
			NetworkDomain: "project:region:default:",
			URI:           "ssh://worker@example.test/workspace?strictHostKeyChecking=true&hostKeyAlias=generation&knownHostsFile=/trusted/hosts&port=22",
			Configuration: map[string]string{"directAddress": address},
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*domain.TransferEndpoint)
	}{
		{"missing generation", func(e *domain.TransferEndpoint) { e.CloudInstanceID = "" }},
		{"same generation", func(e *domain.TransferEndpoint) { e.CloudInstanceID = "source" }},
		{"wrong resolved connection", func(e *domain.TransferEndpoint) { e.ConnectionID = "unbound" }},
		{"other environment", func(e *domain.TransferEndpoint) { e.EnvironmentID = "other" }},
		{"missing network proof", func(e *domain.TransferEndpoint) { e.NetworkDomain = "" }},
		{"different network", func(e *domain.TransferEndpoint) { e.NetworkDomain = "project:region:other:" }},
		{"public address", func(e *domain.TransferEndpoint) { e.Configuration["directAddress"] = "192.0.2.1" }},
		{"missing private address", func(e *domain.TransferEndpoint) { e.Configuration["directAddress"] = "" }},
		{"same private address", func(e *domain.TransferEndpoint) { e.Configuration["directAddress"] = "10.0.0.1" }},
		{"untrusted host", func(e *domain.TransferEndpoint) { e.URI = "ssh://worker@example.test/workspace" }},
		{"agent forwarding", func(e *domain.TransferEndpoint) { e.URI += "&forwardAgent=true" }},
		{"custom port", func(e *domain.TransferEndpoint) {
			e.URI = "ssh://worker@example.test:2222/workspace?strictHostKeyChecking=true&hostKeyAlias=generation&knownHostsFile=/trusted/hosts"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := endpoint("source", "10.0.0.1"), endpoint("destination", "10.0.0.2")
			if !cloudWorkspacePeerEligible(a, b) {
				t.Fatal("expected resolved private peer eligibility before mutation")
			}
			tc.change(&b)
			if cloudWorkspacePeerEligible(a, b) || cloudWorkspacePeerEligible(b, a) {
				t.Fatal("unsafe or unsupported peer must retain fallback")
			}
		})
	}
}
