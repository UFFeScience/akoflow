package terraform

import (
	"testing"
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

func TestProviderNameIsSafe(t *testing.T) {
	if value := providerName("Cloud_INSTANCE_ABC"); value != "cloud-instance-abc" {
		t.Fatalf("unexpected provider name %q", value)
	}
}
