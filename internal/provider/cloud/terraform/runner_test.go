package terraform

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestParseOutput(t *testing.T) {
	result, err := parseOutput([]byte(`{
		"instance_id":{"value":"123"},
		"public_ip":{"value":"203.0.113.8"},
		"private_ip":{"value":"10.0.0.2"},
		"network_domain":{"value":"project:region:network:subnetwork"},
		"disk_name":{"value":"disk-1"},
		"disk_size_gib":{"value":30}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != "123" || result.PublicAddress != "203.0.113.8" || result.Output["network_domain"] != "project:region:network:subnetwork" {
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

func TestApplyRejectsUnsupportedProviderBeforeCreatingWorkspace(t *testing.T) {
	runner := Runner{Root: t.TempDir()}
	_, err := runner.Apply(context.Background(), ports.TerraformProvisionSpec{InstanceID: "cloud-instance-unsupported", Target: domain.CloudCapacityTarget{Provider: "azure"}})
	if err == nil || !strings.Contains(err.Error(), `Terraform provider "azure" is not implemented`) {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
	entries, readErr := os.ReadDir(runner.Root)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("unsupported provider should not create workspace, got %d entries", len(entries))
	}
}

func TestWorkspaceRejectsUnsafeIdentifiers(t *testing.T) {
	runner := Runner{Root: t.TempDir()}
	if _, err := runner.workspace("../escape"); err == nil {
		t.Fatal("expected unsafe instance id to be rejected")
	}
}

func TestParseOutputRejectsInvalidJSON(t *testing.T) {
	if _, err := parseOutput([]byte("not-json")); err == nil {
		t.Fatal("expected malformed terraform output to fail")
	}
}

func TestApplyUsesInjectedExecutorAndWritesPhaseLogs(t *testing.T) {
	var calls [][]string
	runner := Runner{Root: t.TempDir(), Execute: func(_ context.Context, _ string, _ []string, commandAndArguments ...string) ([]byte, error) {
		calls = append(calls, commandAndArguments)
		switch commandAndArguments[1] {
		case "output":
			return []byte(`{"instance_id":{"value":"provider-1"},"public_ip":{"value":"203.0.113.10"}}`), nil
		default:
			return []byte("ok"), nil
		}
	}}
	result, err := runner.Apply(context.Background(), ports.TerraformProvisionSpec{
		InstanceID: "cloud-instance-test-executor",
		Target:     domain.CloudCapacityTarget{Provider: "gcp", ProviderMachineType: "e2-micro", Region: "us-central1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != "provider-1" || result.PublicAddress != "203.0.113.10" {
		t.Fatalf("unexpected terraform result: %#v", result)
	}
	if len(calls) != 4 || calls[0][1] != "fmt" || calls[1][1] != "init" || calls[2][1] != "apply" || calls[3][1] != "output" {
		t.Fatalf("unexpected terraform phases: %#v", calls)
	}
	logData, err := os.ReadFile(filepath.Join(runner.Root, "cloud-instance-test-executor", "provision.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "terraform init") || !strings.Contains(string(logData), "completed") {
		t.Fatalf("expected phase logs, got %q", logData)
	}
}

func TestApplySurfacesInjectedCommandFailure(t *testing.T) {
	runner := Runner{Root: t.TempDir(), Execute: func(_ context.Context, _ string, _ []string, commandAndArguments ...string) ([]byte, error) {
		if commandAndArguments[1] == "init" {
			return []byte("provider download failed"), os.ErrNotExist
		}
		return nil, nil
	}}
	_, err := runner.Apply(context.Background(), ports.TerraformProvisionSpec{InstanceID: "cloud-instance-test-failure", Target: domain.CloudCapacityTarget{Provider: "gcp"}})
	if err == nil || !strings.Contains(err.Error(), "terraform init") || !strings.Contains(err.Error(), "provider download failed") {
		t.Fatalf("expected init failure with command output, got %v", err)
	}
}

func TestPrepareUsesCompatibleDiskForE2Machine(t *testing.T) {
	runner := Runner{Root: t.TempDir()}
	workspace, err := runner.prepare(ports.TerraformProvisionSpec{
		InstanceID: "cloud-instance-disk-compatibility",
		Target: domain.CloudCapacityTarget{
			Provider: "gcp", ProviderMachineType: "e2-micro", Region: "us-central1",
			Configuration: map[string]any{"diskType": "hyperdisk-balanced"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(filepath.Join(workspace, "terraform.tfvars.json"))
	if err != nil {
		t.Fatal(err)
	}
	var variables map[string]any
	if err := json.Unmarshal(encoded, &variables); err != nil {
		t.Fatal(err)
	}
	if variables["disk_type"] != "pd-balanced" {
		t.Fatalf("expected pd-balanced fallback, got %v", variables["disk_type"])
	}
	logData, err := os.ReadFile(filepath.Join(workspace, "provision.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "does not support hyperdisk-balanced") {
		t.Fatalf("expected compatibility decision in log, got %q", string(logData))
	}
}
