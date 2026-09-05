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

func endpoint(uri, instance, network string) domain.TransferEndpoint {
	return domain.TransferEndpoint{URI: uri, CloudInstanceID: instance, NetworkDomain: network, Configuration: map[string]string{}}
}
