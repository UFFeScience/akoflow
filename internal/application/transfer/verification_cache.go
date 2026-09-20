package transfer

import (
	"strings"
	"sync"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// VerifiedArtifactCache remembers immutable, content-addressed objects that
// were verified during this daemon lifetime. A daemon restart intentionally
// requires one fresh destination verification before the cache is trusted.
type VerifiedArtifactCache struct {
	mu       sync.RWMutex
	verified map[string]struct{}
}

func (c *VerifiedArtifactCache) Has(endpoint domain.TransferEndpoint, name, digest string) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.verified[verificationCacheKey(endpoint, name, digest)]
	return ok
}

func (c *VerifiedArtifactCache) Remember(endpoint domain.TransferEndpoint, name, digest string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.verified == nil {
		c.verified = make(map[string]struct{})
	}
	c.verified[verificationCacheKey(endpoint, name, digest)] = struct{}{}
}

func verificationCacheKey(endpoint domain.TransferEndpoint, name, digest string) string {
	return strings.Join([]string{
		endpoint.URI,
		endpoint.ResourceID,
		endpoint.EnvironmentID,
		endpoint.RuntimeID,
		endpoint.ConnectionID,
		endpoint.CloudInstanceID,
		name,
		digest,
	}, "\x00")
}
