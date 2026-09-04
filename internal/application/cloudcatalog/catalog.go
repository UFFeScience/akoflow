package cloudcatalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Catalog struct {
	environments ports.EnvironmentCatalog
	credentials  ports.CloudCredentialResolver
	store        ports.CloudCatalogStore
	providers    map[string]ports.CloudCatalogProvider
}

func New(environments ports.EnvironmentCatalog, credentials ports.CloudCredentialResolver, store ports.CloudCatalogStore, providers ...ports.CloudCatalogProvider) *Catalog {
	registry := make(map[string]ports.CloudCatalogProvider, len(providers))
	for _, provider := range providers {
		registry[provider.Provider()] = provider
	}
	return &Catalog{environments: environments, credentials: credentials, store: store, providers: registry}
}

func (c *Catalog) Discover(ctx context.Context, environmentID string) (domain.CloudCatalog, error) {
	definition, err := c.environments.Find(ctx, strings.TrimSpace(environmentID))
	if err != nil {
		return domain.CloudCatalog{}, err
	}
	if definition == nil {
		return domain.CloudCatalog{}, fmt.Errorf("environment %q was not found", environmentID)
	}
	var connection *domain.EnvironmentConnection
	for index := range definition.Connections {
		if definition.Connections[index].Type == domain.ConnectionCloud {
			connection = &definition.Connections[index]
			break
		}
	}
	if connection == nil {
		return domain.CloudCatalog{}, fmt.Errorf("environment %q has no cloud connection", environmentID)
	}
	provider := strings.ToLower(strings.TrimSpace(stringValue(connection.Configuration, "provider")))
	adapter := c.providers[provider]
	if adapter == nil {
		return domain.CloudCatalog{}, fmt.Errorf("cloud provider %q is not supported", provider)
	}
	credential, err := c.credentials.Resolve(connection.CredentialRef)
	if err != nil {
		return domain.CloudCatalog{}, err
	}
	result, err := c.Validate(ctx, *connection, credential)
	if err != nil {
		return domain.CloudCatalog{}, err
	}
	result.EnvironmentID = environmentID
	if c.store != nil {
		if err := c.store.SaveCloudCatalog(ctx, result); err != nil {
			return domain.CloudCatalog{}, fmt.Errorf("save cloud catalog: %w", err)
		}
	}
	return result, nil
}

func (c *Catalog) Cached(ctx context.Context, environmentID string) (*domain.CloudCatalog, error) {
	if c.store == nil {
		return nil, nil
	}
	return c.store.FindCloudCatalog(ctx, strings.TrimSpace(environmentID))
}

func (c *Catalog) Validate(
	ctx context.Context,
	connection domain.EnvironmentConnection,
	credential []byte,
) (domain.CloudCatalog, error) {
	provider := strings.ToLower(strings.TrimSpace(stringValue(connection.Configuration, "provider")))
	adapter := c.providers[provider]
	if adapter == nil {
		return domain.CloudCatalog{}, fmt.Errorf("cloud provider %q is not supported", provider)
	}
	return adapter.Discover(ctx, connection, credential)
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}
