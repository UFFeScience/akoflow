package slurm

import (
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestInteractiveTerminalUsesConfiguredKnownHosts(t *testing.T) {
	command, err := interactiveCommand(
		domain.EnvironmentConnection{Type: domain.ConnectionSSH, Endpoint: "login.example", Configuration: map[string]any{"knownHostsFile": "/known_hosts/sdumont", "hostKeyAlias": "login.example"}},
		domain.Resource{Type: domain.ResourceHPCPartition, ProviderID: "cpu"},
	)
	if err != nil {
		t.Fatal(err)
	}
	arguments := strings.Join(command.Args, " ")
	if !strings.Contains(arguments, "UserKnownHostsFile=/known_hosts/sdumont") || !strings.Contains(arguments, "StrictHostKeyChecking=yes") {
		t.Fatalf("ssh arguments do not pin configured host keys: %s", arguments)
	}
}

func TestInteractiveSchedulerTerminalCreatesNamedJob(t *testing.T) {
	command, _, err := interactiveCommandForJob(
		domain.EnvironmentConnection{Type: domain.ConnectionSSH, Endpoint: "login.example"},
		domain.Resource{Type: domain.ResourceHPCPartition, ProviderID: "cpu"},
		"akoflow-console-123",
	)
	if err != nil {
		t.Fatal(err)
	}
	arguments := strings.Join(command.Args, " ")
	if !strings.Contains(arguments, "--job-name=akoflow-console-123") {
		t.Fatalf("interactive scheduler job has no unique name: %s", arguments)
	}
}
