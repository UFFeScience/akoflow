package gcp

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type HostVerifier struct {
	Catalog *Catalog
	Root    string
}

func (v HostVerifier) Verify(ctx context.Context, connection domain.EnvironmentConnection, credential []byte, instance domain.CloudProvisionedInstance) (ports.CloudHostTrust, error) {
	if instance.Provider != "gcp" || v.Catalog == nil {
		return ports.CloudHostTrust{}, fmt.Errorf("provider-backed host verification unavailable")
	}
	zone, _ := instance.TerraformOutput["zone"].(string)
	identity, err := v.Catalog.VerifyHostIdentity(ctx, credential, configString(connection.Configuration, "projectId"), zone, instance.Name, instance.ProviderID)
	if err != nil {
		return ports.CloudHostTrust{}, err
	}
	alias, path, err := identity.Enroll(v.Root, instance.ID)
	return ports.CloudHostTrust{Alias: alias, KnownHostsFile: path}, err
}
