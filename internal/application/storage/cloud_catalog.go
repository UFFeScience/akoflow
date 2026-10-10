package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// CloudWorkspaceCatalog resolves a concrete generation, never a logical target
// or an old discovered local endpoint. The root excludes SSH credentials.
type CloudWorkspaceCatalog struct {
	Catalog
	Cloud ports.CloudConfigurationStore
}

func (c CloudWorkspaceCatalog) FindStorage(ctx context.Context, id string) (*domain.StorageResource, error) {
	const prefix = "cloud-workspace:"
	if !strings.HasPrefix(id, prefix) {
		return c.Catalog.FindStorage(ctx, id)
	}
	instanceID := strings.TrimPrefix(id, prefix)
	if c.Cloud == nil || instanceID == "" {
		return nil, fmt.Errorf("cloud workspace identity resolver is unavailable")
	}
	instance, err := c.Cloud.FindProvisionedInstance(ctx, instanceID)
	if err != nil || instance == nil || instance.ID != instanceID || instance.Status != "ready" {
		return nil, fmt.Errorf("concrete cloud workspace instance is unavailable")
	}
	if _, _, err := instance.VerifiedSSHTrust(); err != nil {
		return nil, fmt.Errorf("cloud workspace trust: %w", err)
	}
	return &domain.StorageResource{
		ID: id, Type: domain.StorageSSH, Endpoint: "/akoflow/workspace",
		BrowseRoots: []domain.StorageBrowseRoot{{Path: "/akoflow/workspace"}},
		Metadata:    map[string]any{"cloudInstanceId": instance.ID},
	}, nil
}
