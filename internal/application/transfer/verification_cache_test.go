package transfer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestVerifiedArtifactCacheCoalescesConcurrentMaterializations(t *testing.T) {
	cache := &VerifiedArtifactCache{}
	endpoint := domain.TransferEndpoint{URI: "ssh://cluster/artifacts", ConnectionID: "cluster"}
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32

	results := make(chan bool, 8)
	errorsFound := make(chan error, 8)
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			shared, err := cache.Do(context.Background(), endpoint, "artifact.sif", "sha256:digest", func() error {
				if calls.Add(1) == 1 {
					close(started)
				}
				<-release
				return nil
			})
			results <- shared
			errorsFound <- err
		}()
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("materialization owner did not start")
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	group.Wait()
	close(results)
	close(errorsFound)
	if calls.Load() != 1 {
		t.Fatalf("materialization calls=%d, want 1", calls.Load())
	}
	shared := 0
	for value := range results {
		if value {
			shared++
		}
	}
	if shared != 7 {
		t.Fatalf("shared callers=%d, want 7", shared)
	}
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestVerifiedArtifactCacheReleasesFailedMaterialization(t *testing.T) {
	cache := &VerifiedArtifactCache{}
	endpoint := domain.TransferEndpoint{URI: "ssh://cluster/artifacts"}
	expected := errors.New("upload failed")
	if _, err := cache.Do(context.Background(), endpoint, "artifact.sif", "sha256:digest", func() error {
		return expected
	}); !errors.Is(err, expected) {
		t.Fatalf("first error=%v", err)
	}
	calls := 0
	shared, err := cache.Do(context.Background(), endpoint, "artifact.sif", "sha256:digest", func() error {
		calls++
		return nil
	})
	if err != nil || shared || calls != 1 {
		t.Fatalf("retry shared=%v calls=%d err=%v", shared, calls, err)
	}
}
