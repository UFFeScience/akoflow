package gcp

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	waitCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	identity, err := waitForHostKey(waitCtx, 5*time.Second, func(ctx context.Context) (HostIdentity, error) {
		return v.Catalog.VerifyHostIdentity(ctx, credential, configString(connection.Configuration, "projectId"), zone, instance.Name, instance.ProviderID)
	})
	if err != nil {
		return ports.CloudHostTrust{}, err
	}
	alias, path, err := identity.Enroll(v.Root, instance.ID)
	return ports.CloudHostTrust{Alias: alias, KnownHostsFile: path}, err
}

func waitForHostKey(ctx context.Context, interval time.Duration, read func(context.Context) (HostIdentity, error)) (HostIdentity, error) {
	for {
		identity, err := read(ctx)
		if !errors.Is(err, errHostKeyPublicationPending) {
			return identity, err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return HostIdentity{}, fmt.Errorf("host key publication wait expired: %w", ctx.Err())
		case <-timer.C:
		}
	}
}
