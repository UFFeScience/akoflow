package transfer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPeerSnapshotCacheSharesBuildAndProtectsReaders(t *testing.T) {
	c := &peerSnapshotCache{idleTTL: 10 * time.Millisecond}
	var builds atomic.Int32
	cleaned := make(chan struct{}, 1)
	build := func(context.Context) (*peerSnapshot, error) {
		builds.Add(1)
		return &peerSnapshot{root: "owned-scratch", cleanup: func() { cleaned <- struct{}{} }}, nil
	}
	const readers = 8
	releases := make(chan func(), readers)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snapshot, _, release, err := c.acquire(context.Background(), "sealed-generation", build)
			if err != nil || snapshot.root != "owned-scratch" {
				t.Errorf("acquire: %v", err)
				return
			}
			if snapshot.buildWallSeconds <= 0 {
				t.Error("shared snapshot must publish measured build wall time before readers receive it")
			}
			releases <- release
		}()
	}
	wg.Wait()
	if builds.Load() != 1 {
		t.Fatalf("duplicate builds: %d", builds.Load())
	}
	for i := 0; i < readers-1; i++ {
		(<-releases)()
	}
	select {
	case <-cleaned:
		t.Fatal("deleted while reader active")
	case <-time.After(30 * time.Millisecond):
	}
	last := <-releases
	last()
	last() // Reader release must be idempotent.
	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("idle scratch not cleaned")
	}
	if len(c.entries) != 0 {
		t.Fatal("expired cache entry retained")
	}
}

func TestPeerSnapshotCacheDoesNotCacheFailuresOrUnknownSeals(t *testing.T) {
	c := &peerSnapshotCache{idleTTL: time.Millisecond}
	var builds int
	build := func(context.Context) (*peerSnapshot, error) { builds++; return nil, errors.New("pack failed") }
	for i := 0; i < 2; i++ {
		if _, _, _, err := c.acquire(context.Background(), "seal", build); err == nil {
			t.Fatal("failure suppressed")
		}
	}
	if builds != 2 {
		t.Fatal("failed pack was reused")
	}
	var cleaned atomic.Int32
	for i := 0; i < 2; i++ {
		_, reused, release, err := c.acquire(context.Background(), "", func(context.Context) (*peerSnapshot, error) {
			return &peerSnapshot{cleanup: func() { cleaned.Add(1) }}, nil
		})
		if err != nil || reused {
			t.Fatal("unknown seal reused")
		}
		release()
	}
	if cleaned.Load() != 2 {
		t.Fatal("uncached scratch not released")
	}
}

func TestPeerSnapshotCacheCancelledReaderDoesNotCancelAnother(t *testing.T) {
	c := &peerSnapshotCache{idleTTL: time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	allowBuild := make(chan struct{})
	cleaned := make(chan struct{}, 1)
	build := func(buildCtx context.Context) (*peerSnapshot, error) {
		close(started)
		select {
		case <-allowBuild:
			return &peerSnapshot{cleanup: func() { cleaned <- struct{}{} }}, nil
		case <-buildCtx.Done():
			return nil, buildCtx.Err()
		}
	}
	first := make(chan error, 1)
	go func() {
		_, _, release, err := c.acquire(ctx, "seal", build)
		if release != nil {
			release()
		}
		first <- err
	}()
	<-started
	second := make(chan error, 1)
	go func() {
		_, reused, release, err := c.acquire(context.Background(), "seal", build)
		if err == nil && !reused {
			err = errors.New("did not share build")
		}
		if release != nil {
			release()
		}
		second <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		refs := c.entries["seal"].refs
		c.mu.Unlock()
		if refs == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second reader did not join")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled reader: %v", err)
	}
	close(allowBuild)
	if err := <-second; err != nil {
		t.Fatalf("remaining reader: %v", err)
	}
	select {
	case <-cleaned:
	case <-time.After(time.Second):
		t.Fatal("cleanup missing")
	}
}
