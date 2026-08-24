package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type ConnectionProber struct {
	api              API
	defaultNamespace string
}

func NewConnectionProber(api API, defaultNamespace string) *ConnectionProber {
	if strings.TrimSpace(defaultNamespace) == "" {
		defaultNamespace = "default"
	}
	return &ConnectionProber{api: api, defaultNamespace: defaultNamespace}
}

func (p *ConnectionProber) Probe(
	ctx context.Context,
	connection domain.EnvironmentConnection,
) ports.ConnectionHealth {
	if p.api == nil {
		return ports.ConnectionHealth{Message: "Kubernetes API client is not configured in the engine"}
	}
	namespace := p.defaultNamespace
	if configured, ok := connection.Configuration["namespace"].(string); ok && strings.TrimSpace(configured) != "" {
		namespace = configured
	}
	if _, err := p.api.List(ctx, namespace, "pods", ""); err != nil {
		return ports.ConnectionHealth{Message: fmt.Sprintf("Kubernetes namespace %s is unreachable: %v", namespace, err)}
	}
	return ports.ConnectionHealth{Healthy: true, Message: fmt.Sprintf("Kubernetes namespace %s is reachable", namespace)}
}
