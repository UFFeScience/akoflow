package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type ConnectionProber struct {
	defaultNamespace string
}

func NewConnectionProber(defaultNamespace string) *ConnectionProber {
	if strings.TrimSpace(defaultNamespace) == "" {
		defaultNamespace = "default"
	}
	return &ConnectionProber{defaultNamespace: defaultNamespace}
}

func (p *ConnectionProber) Probe(
	ctx context.Context,
	connection domain.EnvironmentConnection,
) ports.ConnectionHealth {
	namespace := p.defaultNamespace
	if configured, ok := connection.Configuration["namespace"].(string); ok && strings.TrimSpace(configured) != "" {
		namespace = configured
	}
	client, err := NewClient(ClientConfig{
		Endpoint: strings.TrimSpace(connection.Endpoint), Token: configString(connection.Configuration, "bearerToken"),
		CAFile: configString(connection.Configuration, "caFile"), InsecureSkipTLSVerify: configBool(connection.Configuration, "insecureSkipTlsVerify", false),
	})
	if err != nil {
		return ports.ConnectionHealth{Message: fmt.Sprintf("Kubernetes connection is invalid: %v", err)}
	}
	if _, err := client.List(ctx, namespace, "pods", ""); err != nil {
		return ports.ConnectionHealth{Message: fmt.Sprintf("Kubernetes namespace %s is unreachable: %v", namespace, err)}
	}
	return ports.ConnectionHealth{Healthy: true, Message: fmt.Sprintf("Kubernetes namespace %s is reachable", namespace)}
}
