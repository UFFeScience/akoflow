package transfer

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type EnvironmentEndpointResolver struct {
	Connections ports.ConnectionStore
}

func (resolver EnvironmentEndpointResolver) ResolveTransferEndpoint(ctx context.Context, location domain.TransferLocation) (domain.TransferEndpoint, error) {
	endpoint := domain.TransferEndpoint{URI: location.URI}
	u, err := url.Parse(location.URI)
	if err != nil {
		return endpoint, err
	}
	connectionID := u.Query().Get("connectionId")
	if connectionID == "" && u.Scheme == "kubernetes" {
		connectionID = u.Host
		u.Host = ""
		endpoint.URI = u.String()
	}
	if connectionID == "" {
		return endpoint, nil
	}
	if resolver.Connections == nil {
		return endpoint, fmt.Errorf("transfer endpoint %q requires an environment connection store", location.URI)
	}
	connection, err := resolver.Connections.FindConnection(ctx, connectionID)
	if err != nil || connection == nil {
		return endpoint, fmt.Errorf("resolve transfer connection %q: %w", connectionID, err)
	}
	endpoint.ID = connection.ID
	endpoint.EnvironmentID = connection.EnvironmentID
	endpoint.Configuration = make(map[string]string)
	switch connection.Type {
	case domain.ConnectionKubernetes:
		endpoint.Configuration["server"] = connection.Endpoint
		endpoint.Configuration["tokenFile"] = strings.TrimPrefix(connection.CredentialRef, "file:")
		for _, key := range []string{"caFile"} {
			if value, ok := connection.Configuration[key].(string); ok {
				endpoint.Configuration[key] = value
			}
		}
		if value, ok := connection.Configuration["insecureSkipTlsVerify"].(bool); ok {
			endpoint.Configuration["insecureSkipTLSVerify"] = strconv.FormatBool(value)
		}
	case domain.ConnectionSSH, domain.ConnectionAgent:
		remote := &url.URL{Scheme: "ssh", Host: connection.Endpoint, Path: u.Path}
		if connection.Username != "" {
			remote.User = url.User(connection.Username)
		}
		query := remote.Query()
		switch value := connection.Configuration["port"].(type) {
		case float64:
			query.Set("port", strconv.Itoa(int(value)))
		case int:
			query.Set("port", strconv.Itoa(value))
		}
		if value, ok := connection.Configuration["proxyCommand"].(string); ok && value != "" {
			query.Set("proxyCommand", value)
		}
		knownHosts, _ := connection.Configuration["knownHostsFile"].(string)
		if knownHosts == "" {
			knownHosts = "storage/credentials/ssh/known_hosts"
		}
		query.Set("knownHostsFile", knownHosts)
		if value, ok := connection.Configuration["forwardAgent"].(bool); ok && value {
			query.Set("forwardAgent", "true")
		}
		if key := strings.TrimPrefix(connection.CredentialRef, "file:"); key != "" {
			query.Set("identityFile", key)
		}
		remote.RawQuery = query.Encode()
		endpoint.URI = remote.String()
	default:
		return endpoint, fmt.Errorf("connection %q of type %q cannot resolve URI scheme %q", connectionID, connection.Type, u.Scheme)
	}
	return endpoint, nil
}
