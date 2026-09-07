package transfer

import (
	"net/url"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// StrategyResolver classifies the topology without moving bytes. Keeping this
// policy independent makes every runtime combination testable without access
// to a real cloud, cluster, or HPC login node.
type StrategyResolver struct{}

func (StrategyResolver) Resolve(source, destination domain.TransferEndpoint) domain.TransferRoute {
	sourceAddress := endpointAddress(source)
	targetAddress := endpointAddress(destination)
	route := domain.TransferRoute{
		SourceAddress:         sourceAddress,
		TargetAddress:         targetAddress,
		SourceCloudInstanceID: source.CloudInstanceID,
		TargetCloudInstanceID: destination.CloudInstanceID,
		Fallback:              domain.TransferGateway,
	}
	if sameObject(source, destination) {
		route.Strategy = domain.TransferUseExisting
		route.Reason = "source and destination refer to the same committed object"
		return route
	}
	if source.CloudInstanceID != "" && source.CloudInstanceID == destination.CloudInstanceID {
		route.Strategy = domain.TransferRuntimeLocal
		route.NetworkDomain = source.NetworkDomain
		route.Reason = "source and destination are workspaces on the same cloud instance"
		return route
	}
	if shared := source.Configuration["sharedStorageId"]; shared != "" && shared == destination.Configuration["sharedStorageId"] {
		route.Strategy = domain.TransferSharedStorage
		route.Reason = "both endpoints expose the same verified shared storage"
		return route
	}
	if source.NetworkDomain != "" && source.NetworkDomain == destination.NetworkDomain &&
		strings.HasPrefix(source.URI, "ssh://") && strings.HasPrefix(destination.URI, "ssh://") {
		route.Strategy = domain.TransferDirectRuntime
		route.NetworkDomain = source.NetworkDomain
		route.Reason = "SSH endpoints share a directly reachable network domain"
		return route
	}
	route.Strategy = domain.TransferGateway
	route.Fallback = ""
	route.Reason = "runtime endpoints have no verified direct route; Akoflow relay is required"
	return route
}

func sameObject(source, destination domain.TransferEndpoint) bool {
	return source.URI != "" && source.URI == destination.URI
}

func endpointAddress(endpoint domain.TransferEndpoint) string {
	u, err := url.Parse(endpoint.URI)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
