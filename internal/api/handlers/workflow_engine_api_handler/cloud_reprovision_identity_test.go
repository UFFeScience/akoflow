package workflow_engine_api_handler

import (
	"strings"
	"testing"
)

func TestProvisionOperationPreservesInstanceIdentity(t *testing.T) {
	for _, tc := range []struct{ existing, requested, want string }{
		{"", "cloud-instance-original", "cloud-instance-original"},
		{"cloud-instance-original", "cloud-instance-original", "cloud-instance-original"},
		{"", " cloud-instance-original ", "cloud-instance-original"},
	} {
		if got := provisionOperationInstanceID(tc.existing, tc.requested); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
	first, second := provisionOperationInstanceID("", ""), provisionOperationInstanceID("", "")
	if !strings.HasPrefix(first, "cloud-instance-") || first == second {
		t.Fatal("fresh provisioning must generate distinct instance identities")
	}
}
