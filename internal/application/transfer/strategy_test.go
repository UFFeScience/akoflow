package transfer

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestStrategyResolverClassifiesRuntimeTopology(t *testing.T) {
	tests := []struct {
		name        string
		source      domain.TransferEndpoint
		destination domain.TransferEndpoint
		want        domain.TransferStrategy
	}{
		{name: "existing", source: endpoint("ssh://a/work", "a", "vpc"), destination: endpoint("ssh://a/work", "a", "vpc"), want: domain.TransferUseExisting},
		{name: "same vm", source: endpoint("ssh://a/one", "a", "vpc"), destination: endpoint("ssh://a/two", "a", "vpc"), want: domain.TransferRuntimeLocal},
		{name: "direct cloud", source: endpoint("ssh://10.0.0.1/one", "a", "vpc"), destination: endpoint("ssh://10.0.0.2/two", "b", "vpc"), want: domain.TransferDirectRuntime},
		{name: "isolated", source: endpoint("ssh://a/one", "a", "gcp"), destination: endpoint("ssh://b/two", "b", "aws"), want: domain.TransferGateway},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			route := (StrategyResolver{}).Resolve(test.source, test.destination)
			if route.Strategy != test.want || route.Reason == "" {
				t.Fatalf("Resolve() = %#v, want %q with an explanation", route, test.want)
			}
		})
	}
}

func TestStrategyResolverCoversHybridCloudAcceptanceRoute(t *testing.T) {
	resolver := StrategyResolver{}
	kubernetes := endpoint("kubernetes://cluster/workspace", "", "kubernetes")
	cloudA := endpoint("ssh://worker-a/workspace-a", "cloud-a", "gcp-vpc")
	cloudAConsumer := endpoint("ssh://worker-a/workspace-b", "cloud-a", "gcp-vpc")
	cloudB := endpoint("ssh://worker-b/workspace", "cloud-b", "gcp-vpc")
	hpc := endpoint("ssh://plafrim/workspace", "", "plafrim")
	cases := []struct {
		name        string
		source      domain.TransferEndpoint
		destination domain.TransferEndpoint
		expected    domain.TransferStrategy
	}{
		{
			name: "kubernetes to cloud A", source: kubernetes,
			destination: cloudA, expected: domain.TransferGateway,
		},
		{
			name: "within cloud A", source: cloudA,
			destination: cloudAConsumer, expected: domain.TransferRuntimeLocal,
		},
		{
			name: "cloud A to cloud B", source: cloudA,
			destination: cloudB, expected: domain.TransferDirectRuntime,
		},
		{
			name: "cloud B to HPC", source: cloudB,
			destination: hpc, expected: domain.TransferGateway,
		},
		{
			name: "HPC to cloud B", source: hpc,
			destination: cloudB, expected: domain.TransferGateway,
		},
		{
			name: "cloud B to Kubernetes", source: cloudB,
			destination: kubernetes, expected: domain.TransferGateway,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			route := resolver.Resolve(test.source, test.destination)
			if route.Strategy != test.expected || route.Reason == "" {
				t.Fatalf("route=%#v", route)
			}
		})
	}
}

func TestStrategyResolverReportsPrivateAddressForDirectCloudRoute(t *testing.T) {
	source := endpoint("ssh://user@203.0.113.10/work", "cloud-a", "vpc")
	destination := endpoint("ssh://user@203.0.113.11/work", "cloud-b", "vpc")
	source.Configuration["directAddress"] = "10.0.0.10"
	destination.Configuration["directAddress"] = "10.0.0.11"
	route := (StrategyResolver{}).Resolve(source, destination)
	if route.Strategy != domain.TransferDirectRuntime || route.SourceAddress != "10.0.0.10" || route.TargetAddress != "10.0.0.11" {
		t.Fatalf("route=%#v", route)
	}
}

func endpoint(uri, instance, network string) domain.TransferEndpoint {
	return domain.TransferEndpoint{URI: uri, CloudInstanceID: instance, NetworkDomain: network, Configuration: map[string]string{}}
}
