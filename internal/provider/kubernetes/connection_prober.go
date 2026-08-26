package kubernetes

import (
	"context"
	"fmt"
	"os"
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
	token, err := connectionToken(connection)
	if err != nil {
		return ports.ConnectionHealth{Message: fmt.Sprintf("Kubernetes credential is invalid: %v", err)}
	}
	client, err := NewClient(ClientConfig{
		Endpoint: strings.TrimSpace(connection.Endpoint), Token: token,
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

func connectionToken(connection domain.EnvironmentConnection) (string, error) {
	reference := strings.TrimSpace(connection.CredentialRef)
	if strings.HasPrefix(reference, "env:") {
		name := strings.TrimSpace(strings.TrimPrefix(reference, "env:"))
		if name == "" {
			return "", fmt.Errorf("empty environment credential name")
		}
		if token := os.Getenv(name); token != "" {
			return token, nil
		}
		return "", fmt.Errorf("environment credential %q is not set", name)
	}
	if strings.HasPrefix(reference, "file:") {
		path := strings.TrimSpace(strings.TrimPrefix(reference, "file:"))
		if path == "" {
			return "", fmt.Errorf("empty file credential path")
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read Kubernetes credential: %w", err)
		}
		if token := strings.TrimSpace(string(value)); token != "" {
			return token, nil
		}
		return "", fmt.Errorf("Kubernetes credential file is empty")
	}
	return configString(connection.Configuration, "bearerToken"), nil
}
