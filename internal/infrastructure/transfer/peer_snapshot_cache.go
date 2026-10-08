package transfer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type peerSnapshot struct {
	root, digest string
	size         int64
	cleanup      func()
}

type peerSnapshotEntry struct {
	done     chan struct{}
	cancel   context.CancelFunc
	refs     int
	snapshot *peerSnapshot
	err      error
	timer    *time.Timer
}

// The cache owns remote scratch directories, never producer workspaces. Active
// readers prevent eviction. Idle entries expire; failed builds are not cached.
type peerSnapshotCache struct {
	mu      sync.Mutex
	entries map[string]*peerSnapshotEntry
	idleTTL time.Duration
}

var sharedPeerSnapshots = peerSnapshotCache{entries: make(map[string]*peerSnapshotEntry), idleTTL: time.Minute}

func (c *peerSnapshotCache) acquire(ctx context.Context, key string, build func(context.Context) (*peerSnapshot, error)) (*peerSnapshot, bool, func(), error) {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*peerSnapshotEntry)
	}
	e := c.entries[key]
	reused := e != nil && key != ""
	if key == "" {
		e = nil
	}
	if e == nil {
		if key != "" && len(c.entries) >= 64 {
			c.mu.Unlock()
			return nil, false, nil, fmt.Errorf("peer snapshot cache capacity exhausted")
		}
		buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Minute)
		e = &peerSnapshotEntry{done: make(chan struct{}), cancel: cancel}
		if key != "" {
			c.entries[key] = e
		}
		go func() {
			snapshot, err := build(buildCtx)
			cancel()
			c.mu.Lock()
			e.snapshot, e.err = snapshot, err
			close(e.done)
			if err != nil && c.entries[key] == e {
				delete(c.entries, key)
			}
			orphaned := e.refs == 0
			c.mu.Unlock()
			if orphaned && snapshot != nil && snapshot.cleanup != nil {
				snapshot.cleanup()
			}
		}()
	}
	e.refs++
	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}
	c.mu.Unlock()
	var once sync.Once
	release := func() { once.Do(func() { c.release(key, e) }) }
	select {
	case <-ctx.Done():
		release()
		return nil, reused, nil, ctx.Err()
	case <-e.done:
		if e.err != nil {
			release()
			return nil, reused, nil, e.err
		}
		return e.snapshot, reused, release, nil
	}
}

func (c *peerSnapshotCache) release(key string, e *peerSnapshotEntry) {
	c.mu.Lock()
	e.refs--
	if e.refs != 0 {
		c.mu.Unlock()
		return
	}
	select {
	case <-e.done:
		if key != "" && e.err == nil && c.entries[key] == e {
			e.timer = time.AfterFunc(c.idleTTL, func() {
				c.mu.Lock()
				if e.refs != 0 || c.entries[key] != e {
					c.mu.Unlock()
					return
				}
				delete(c.entries, key)
				c.mu.Unlock()
				if e.snapshot != nil && e.snapshot.cleanup != nil {
					e.snapshot.cleanup()
				}
			})
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
		if e.snapshot != nil && e.snapshot.cleanup != nil {
			e.snapshot.cleanup()
		}
	default:
		if c.entries[key] == e {
			delete(c.entries, key)
		}
		e.cancel()
		c.mu.Unlock()
	}
}
