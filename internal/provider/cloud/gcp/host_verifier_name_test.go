package gcp

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestHostVerifierUsesProviderNameInsteadOfDisplayLabel(t *testing.T) {
	instance := domain.CloudProvisionedInstance{Name: "Friendly display name", TerraformOutput: map[string]any{"instance_name": "cloud-instance-actual"}}
	if got := providerInstanceName(instance); got != "cloud-instance-actual" {
		t.Fatal(got)
	}
	instance.TerraformOutput = nil
	if got := providerInstanceName(instance); got != instance.Name {
		t.Fatal("legacy compatibility", got)
	}
}
