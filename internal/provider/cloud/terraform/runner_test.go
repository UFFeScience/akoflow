package terraform

import (
	"context"
	"os/exec"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestParseOutput(t *testing.T) {
	result, err := parseOutput([]byte(`{
		"instance_id":{"value":"123"},
		"public_ip":{"value":"203.0.113.8"},
		"private_ip":{"value":"10.0.0.2"},
		"disk_name":{"value":"disk-1"},
		"disk_size_gib":{"value":30}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != "123" || result.PublicAddress != "203.0.113.8" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestGeneratedGCPModuleIsFormatted(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform is not installed")
	}
	runner := Runner{Root: t.TempDir(), Binary: "terraform"}
	workspace, err := runner.prepare(ports.TerraformProvisionSpec{
		InstanceID: "cloud-instance-test",
		Target: domain.CloudCapacityTarget{
			Provider: "gcp", ProviderMachineType: "e2-small", Region: "us-central1",
			ImageReference: "projects/debian-cloud/global/images/family/debian-12",
			Configuration:  map[string]any{"projectId": "test"},
		},
		Credential: []byte(`{}`), PublicKey: "ssh-ed25519 test", SSHUser: "akoflow",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.run(context.Background(), workspace, "fmt"); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.run(context.Background(), workspace, "fmt", "-check"); err != nil {
		t.Fatal(err)
	}
}

func TestProviderNameIsSafe(t *testing.T) {
	if value := providerName("Cloud_INSTANCE_ABC"); value != "cloud-instance-abc" {
		t.Fatalf("unexpected provider name %q", value)
	}
}
