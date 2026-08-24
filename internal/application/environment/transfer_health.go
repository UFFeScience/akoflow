package environment

import (
	"context"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// BindingProbe is registered by the deployment for a connector. It keeps
// object-store SDKs and credentials out of infrastructure discovery.
type BindingProbe func(context.Context, domain.ConnectorBinding) (string, error)

// ConnectorBindingChecker performs one explicitly requested check. It does not
// enumerate bindings, preventing surprise credential access during discovery.
type ConnectorBindingChecker struct {
	probes map[domain.TransferConnector]BindingProbe
	now    func() time.Time
}

func NewConnectorBindingChecker(probes map[domain.TransferConnector]BindingProbe) *ConnectorBindingChecker {
	return &ConnectorBindingChecker{probes: probes, now: time.Now}
}

func (s *ConnectorBindingChecker) CheckConnectorBinding(ctx context.Context, binding domain.ConnectorBinding) (domain.ConnectorHealth, error) {
	probe := s.probes[binding.Connector]
	if probe == nil {
		return domain.ConnectorHealth{Reason: "no health probe is configured", CheckedAt: s.now().UTC()}, fmt.Errorf("no health probe for connector %q", binding.Connector)
	}
	op, err := probe(ctx, binding)
	health := domain.ConnectorHealth{Healthy: err == nil, Operation: op, CheckedAt: s.now().UTC(), FreshUntil: s.now().UTC().Add(15 * time.Minute)}
	if err != nil {
		health.Reason = err.Error()
	}
	return health, err
}
