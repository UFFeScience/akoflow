package kubernetes

import (
	"fmt"
	"os"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// ConnectionFactory creates an isolated Kubernetes client for each persisted
// environment connection. A server can therefore operate against more than
// one cluster in the same process.
type ConnectionFactory struct {
	DefaultNamespace string
	Fallback         ClientConfig
}

func (ConnectionFactory) Driver() domain.RuntimeDriver { return domain.RuntimeDriverKubernetes }

func (f ConnectionFactory) Build(runtime domain.EnvironmentRuntime, connection domain.EnvironmentConnection) (ports.RuntimeAdapter, error) {
	if connection.Type != domain.ConnectionKubernetes {
		return nil, fmt.Errorf("connection %q is %q, Kubernetes runtime requires a kubernetes connection", connection.ID, connection.Type)
	}
	endpoint := strings.TrimSpace(connection.Endpoint)
	if endpoint == "" {
		endpoint = f.Fallback.Endpoint
	}
	token, err := resolveCredential(connection.CredentialRef)
	if err != nil {
		return nil, fmt.Errorf("resolve credential for connection %q: %w", connection.ID, err)
	}
	if token == "" {
		token = f.Fallback.Token
	}
	if endpoint == "" || token == "" {
		return nil, fmt.Errorf("connection %q needs an endpoint and a credential reference (env:VARIABLE)", connection.ID)
	}
	namespace := configString(runtime.Configuration, "namespace")
	if namespace == "" {
		namespace = configString(connection.Configuration, "namespace")
	}
	if namespace == "" {
		namespace = f.DefaultNamespace
	}
	insecure := configBool(connection.Configuration, "insecureSkipTlsVerify", f.Fallback.InsecureSkipTLSVerify)
	client, err := NewClient(ClientConfig{Endpoint: endpoint, Token: token,
		CAFile: configString(connection.Configuration, "caFile"), InsecureSkipTLSVerify: insecure})
	if err != nil {
		return nil, err
	}
	return New(client, namespace), nil
}

func resolveCredential(reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	if strings.HasPrefix(reference, "env:") {
		name := strings.TrimSpace(strings.TrimPrefix(reference, "env:"))
		if name == "" {
			return "", fmt.Errorf("empty environment credential name")
		}
		value := os.Getenv(name)
		if value == "" {
			return "", fmt.Errorf("environment credential %q is not set", name)
		}
		return value, nil
	}
	// Existing keychain and secret-manager references are intentionally not
	// interpreted here. Their resolver will be injected when those providers
	// are introduced; the fallback preserves the current Kind development setup.
	return "", nil
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
