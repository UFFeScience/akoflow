package kubernetes

import (
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// ConnectionFactory creates an isolated Kubernetes client for each persisted
// environment connection. A server can therefore operate against more than
// one cluster in the same process.
type ConnectionFactory struct {
	DefaultNamespace string
}

func (ConnectionFactory) Driver() domain.RuntimeDriver { return domain.RuntimeDriverKubernetes }

func (f ConnectionFactory) Build(runtime domain.EnvironmentRuntime, connection domain.EnvironmentConnection) (ports.RuntimeAdapter, error) {
	if connection.Type != domain.ConnectionKubernetes {
		return nil, fmt.Errorf("connection %q is %q, Kubernetes runtime requires a kubernetes connection", connection.ID, connection.Type)
	}
	endpoint := strings.TrimSpace(connection.Endpoint)
	token, err := connectionToken(connection)
	if err != nil {
		return nil, fmt.Errorf("connection %q credential: %w", connection.ID, err)
	}
	if endpoint == "" || token == "" {
		return nil, fmt.Errorf("connection %q needs an endpoint and bearer token", connection.ID)
	}
	namespace := configString(runtime.Configuration, "namespace")
	if namespace == "" {
		namespace = configString(connection.Configuration, "namespace")
	}
	if namespace == "" {
		namespace = f.DefaultNamespace
	}
	insecure := configBool(connection.Configuration, "insecureSkipTlsVerify", false)
	client, err := NewClient(ClientConfig{Endpoint: endpoint, Token: token,
		CAFile: configString(connection.Configuration, "caFile"), InsecureSkipTLSVerify: insecure})
	if err != nil {
		return nil, err
	}
	return New(client, namespace), nil
}

func configString(configuration map[string]any, key string) string {
	if value, ok := configuration[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func configBool(configuration map[string]any, key string, fallback bool) bool {
	if value, ok := configuration[key].(bool); ok {
		return value
	}
	return fallback
}
