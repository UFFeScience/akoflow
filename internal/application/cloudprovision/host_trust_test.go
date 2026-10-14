package cloudprovision

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type hostVerifierStub struct {
	err   error
	calls int
}

func (v *hostVerifierStub) Verify(_ context.Context, _ domain.EnvironmentConnection, _ []byte, instance domain.CloudProvisionedInstance) (ports.CloudHostTrust, error) {
	v.calls++
	if instance.ProviderID != "123" {
		return ports.CloudHostTrust{}, errors.New("wrong generation supplied to verifier")
	}
	return ports.CloudHostTrust{Alias: "generation-123", KnownHostsFile: "verified-hosts"}, v.err
}

func TestProvisionRequiresHostVerificationBeforeReady(t *testing.T) {
	for _, reject := range []bool{false, true} {
		store := &capacityStoreStub{target: &domain.CloudCapacityTarget{ID: "target", EnvironmentID: "environment", Provider: "gcp"}}
		environment := &environmentCatalogStub{definition: domain.EnvironmentDefinition{Connections: []domain.EnvironmentConnection{{Type: domain.ConnectionCloud, CredentialRef: "credential"}}}}
		verifier := &hostVerifierStub{}
		if reject {
			verifier.err = errors.New("unverified host key")
		}
		service := New(store, environment, credentialStub{}, sshKeyStub{}, &terraformStub{result: ports.TerraformResult{ProviderID: "123", PublicAddress: "203.0.113.10"}}, configuratorStub{}).WithHostTrustVerifier(verifier)
		instance, err := service.Provision(context.Background(), "environment", domain.CloudProvisionRequest{CapacityTargetID: "target"})
		if verifier.calls != 1 {
			t.Fatalf("verification calls=%d", verifier.calls)
		}
		if reject {
			if err == nil || instance.Status != "failed" {
				t.Fatalf("unverified instance was accepted: %s %v", instance.Status, err)
			}
		} else {
			if err != nil || instance.Status != "ready" {
				t.Fatalf("verified provision: %s %v", instance.Status, err)
			}
			if _, _, err := instance.VerifiedSSHTrust(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
