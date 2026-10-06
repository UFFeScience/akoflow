package transfer

import (
	"context"
	"errors"
	"fmt"
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
	for node := range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			nodeEndpoint := endpoint
			nodeEndpoint.ResourceID = fmt.Sprintf("scheduler-node-%d", node)
			shared, err := cache.Do(context.Background(), nodeEndpoint, "artifact.sif", "sha256:digest", func() error {
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

func TestVerifiedArtifactCacheSharesSSHArtifactAcrossSchedulerNodes(t *testing.T) {
	cache := &VerifiedArtifactCache{}
	first := domain.TransferEndpoint{URI: "ssh://cluster/artifacts", ConnectionID: "cluster", ResourceID: "diablo03", RuntimeID: "slurm"}
	second := first
	second.ResourceID = "bora001"
	cache.Remember(first, "artifact.sif", "sha256:digest")
	shared, err := cache.Do(context.Background(), second, "artifact.sif", "sha256:digest", func() error {
		return errors.New("must not write the shared partial file again")
	})
	if err != nil || !shared {
		t.Fatalf("shared=%v err=%v", shared, err)
	}
	second.CloudInstanceID = "different-vm"
	if cache.Has(second, "artifact.sif", "sha256:digest") {
		t.Fatal("different VMs must not share verification")
	}
}

func TestVerifiedArtifactCacheReportsInFlightOwnerBeforeWaiting(t *testing.T) {
	cache := &VerifiedArtifactCache{}
	endpoint := domain.TransferEndpoint{URI: "ssh://cluster/artifacts"}
	ownerStarted := make(chan struct{})
	releaseOwner := make(chan struct{})
	ownerDone := make(chan error, 1)
	go func() {
		_, err := cache.DoObserved(context.Background(), endpoint, "artifact.sif", "sha256:digest",
			"activity-owner", nil, func() error {
				close(ownerStarted)
				<-releaseOwner
				return nil
			})
		ownerDone <- err
	}()
	<-ownerStarted
	waitObserved := make(chan string, 1)
	followerDone := make(chan error, 1)
	go func() {
		shared, err := cache.DoObserved(context.Background(), endpoint, "artifact.sif", "sha256:digest",
			"activity-follower", func(owner string) { waitObserved <- owner }, func() error {
				return errors.New("follower unexpectedly became owner")
			})
		if err == nil && !shared {
			err = errors.New("follower result was not shared")
		}
		followerDone <- err
	}()
	select {
	case owner := <-waitObserved:
		if owner != "activity-owner" {
			t.Fatalf("owner=%q", owner)
		}
	case <-time.After(time.Second):
		t.Fatal("follower did not report waiting")
	}
	close(releaseOwner)
	if err := <-ownerDone; err != nil {
		t.Fatal(err)
	}
	if err := <-followerDone; err != nil {
		t.Fatal(err)
	}
}
