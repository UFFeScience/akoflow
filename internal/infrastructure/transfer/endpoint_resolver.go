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
	Cloud       ports.CloudConfigurationStore
	Provisioner ports.CloudProvisioner
}

func (resolver EnvironmentEndpointResolver) ResolveTransferEndpoint(ctx context.Context, location domain.TransferLocation) (domain.TransferEndpoint, error) {
	endpoint := domain.TransferEndpoint{
		URI: location.URI, ResourceID: location.ResourceID, EnvironmentID: location.EnvironmentID,
		RuntimeID: location.RuntimeID, ConnectionID: location.ConnectionID,
		CloudInstanceID: location.CloudInstanceID, NetworkDomain: location.NetworkDomain,
	}
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
	if connection.Type == domain.ConnectionCloud {
		connection, err = resolver.cloudConnection(ctx, *connection, location.ResourceID, location.CloudInstanceID)
		if err != nil {
			return endpoint, err
		}
	}
	endpoint.ID = connection.ID
	endpoint.ConnectionID = connection.ID
	endpoint.EnvironmentID = connection.EnvironmentID
	if value, _ := connection.Configuration["networkDomain"].(string); value != "" {
		endpoint.NetworkDomain = value
	}
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
		if value, _ := connection.Configuration["acceptNewHostKey"].(bool); value {
			query.Set("acceptNewHostKey", "true")
		}
		if value, _ := connection.Configuration["hostKeyAlias"].(string); value != "" {
			query.Set("hostKeyAlias", value)
		}
		if value, ok := connection.Configuration["forwardAgent"].(bool); ok && value {
			query.Set("forwardAgent", "true")
		}
		if key := strings.TrimPrefix(connection.CredentialRef, "file:"); key != "" {
			query.Set("identityFile", key)
		}
		if value, _ := connection.Configuration["directAddress"].(string); value != "" {
			endpoint.Configuration["directAddress"] = value
		}
		remote.RawQuery = query.Encode()
		endpoint.URI = remote.String()
	default:
		return endpoint, fmt.Errorf("connection %q of type %q cannot resolve URI scheme %q", connectionID, connection.Type, u.Scheme)
	}
	return endpoint, nil
}

func (resolver EnvironmentEndpointResolver) cloudConnection(
	ctx context.Context,
	connection domain.EnvironmentConnection,
	capacityTargetID string,
	cloudInstanceID string,
) (*domain.EnvironmentConnection, error) {
	if resolver.Cloud == nil {
		return nil, fmt.Errorf("cloud transfer endpoint is unavailable")
	}
	if strings.TrimSpace(cloudInstanceID) == "" {
		return nil, fmt.Errorf("cloud transfer endpoint for capacity target %q has no concrete instance allocation", capacityTargetID)
	}
	instances, err := resolver.Cloud.ListProvisionedInstances(ctx, connection.EnvironmentID)
	if err != nil {
		return nil, err
	}
	var selected *domain.CloudProvisionedInstance
	for index := range instances {
		if instances[index].ID == cloudInstanceID && instances[index].Status == "ready" && instances[index].CapacityTargetID == capacityTargetID {
			selected = &instances[index]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("allocated cloud instance %q is not ready for capacity target %q", cloudInstanceID, capacityTargetID)
	}
	address := selected.PublicAddress
	if strings.TrimSpace(address) == "" {
		address = selected.PrivateAddress
	}
	networkDomain, _ := selected.TerraformOutput["network_domain"].(string)
	if networkDomain == "" {
		networkDomain = selected.EnvironmentID
	}
	return &domain.EnvironmentConnection{
		ID: connection.ID + "-" + selected.ID, EnvironmentID: connection.EnvironmentID,
		Name: selected.Name, Type: domain.ConnectionSSH, Endpoint: address,
		Username: selected.SSHUsername, CredentialRef: selected.SSHCredentialRef,
		Configuration: map[string]any{
			"port": 22, "acceptNewHostKey": true,
			"hostKeyAlias": selected.ID, "networkDomain": networkDomain,
			"directAddress": selected.PrivateAddress,
		},
	}, nil
}
