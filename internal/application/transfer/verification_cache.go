package transfer

import (
	"context"
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
	inFlight map[string]*artifactFlight
}

type artifactFlight struct {
	done chan struct{}
	err  error
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

// Do ensures that only one caller materializes a content-addressed object at a
// destination. Concurrent callers wait for the owner and reuse its verified
// result instead of writing to the same partial file.
func (c *VerifiedArtifactCache) Do(
	ctx context.Context,
	endpoint domain.TransferEndpoint,
	name string,
	digest string,
	materialize func() error,
) (bool, error) {
	if c == nil {
		return false, materialize()
	}
	key := verificationCacheKey(endpoint, name, digest)
	c.mu.Lock()
	if _, ok := c.verified[key]; ok {
		c.mu.Unlock()
		return true, nil
	}
	if flight := c.inFlight[key]; flight != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case <-flight.done:
			return true, flight.err
		}
	}
	if c.inFlight == nil {
		c.inFlight = make(map[string]*artifactFlight)
	}
	flight := &artifactFlight{done: make(chan struct{})}
	c.inFlight[key] = flight
	c.mu.Unlock()

	err := materialize()
	c.mu.Lock()
	flight.err = err
	if err == nil {
		if c.verified == nil {
			c.verified = make(map[string]struct{})
		}
		c.verified[key] = struct{}{}
	}
	delete(c.inFlight, key)
	close(flight.done)
	c.mu.Unlock()
	return false, err
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
