package transfer

import (
	"net/netip"
	"net/url"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// cloudWorkspacePeerEligible is only an eligibility gate, not proof of
// connectivity. EndpointResolver supplies the concrete generation, private
// address and Terraform network identity. A subsequent authenticated probe
// must prove reachability before any peer payload transfer is selected.
func cloudWorkspacePeerEligible(source, destination domain.TransferEndpoint) bool {
	if source.CloudInstanceID == "" || destination.CloudInstanceID == "" || source.CloudInstanceID == destination.CloudInstanceID {
		return false
	}
	if source.EnvironmentID == "" || source.EnvironmentID != destination.EnvironmentID ||
		source.NetworkDomain == "" || source.NetworkDomain != destination.NetworkDomain {
		return false
	}
	addresses := make([]netip.Addr, 0, 2)
	for _, endpoint := range []domain.TransferEndpoint{source, destination} {
		if endpoint.ConnectionID == "" || !strings.HasSuffix(endpoint.ConnectionID, "-"+endpoint.CloudInstanceID) {
			return false
		}
		u, err := url.Parse(endpoint.URI)
		if err != nil || u.Scheme != "ssh" || u.User == nil || u.User.Username() == "" {
			return false
		}
		q := u.Query()
		if q.Get("strictHostKeyChecking") != "true" || q.Get("hostKeyAlias") == "" || q.Get("knownHostsFile") == "" ||
			q.Get("acceptNewHostKey") == "true" || q.Get("forwardAgent") == "true" {
			return false
		}
		// The current cloud connector resolves port 22. Do not silently ignore
		// a custom port when constructing a private peer connection.
		if (u.Port() != "" && u.Port() != "22") || (q.Get("port") != "" && q.Get("port") != "22") {
			return false
		}
		address, err := netip.ParseAddr(strings.TrimSpace(endpoint.Configuration["directAddress"]))
		if err != nil || !address.Is4() || !address.IsPrivate() {
			return false
		}
		addresses = append(addresses, address)
	}
	return addresses[0] != addresses[1]
}
