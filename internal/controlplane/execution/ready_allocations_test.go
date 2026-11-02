package execution

import (
	"errors"
	"testing"
)

func TestDrainReadyAllocationsBatchesAvailableWithoutWaiting(t *testing.T) {
	results := make(chan allocationResult, 8)
	for i := 0; i < 8; i++ {
		results <- allocationResult{activityID: "ready"}
	}
	consumed := 0
	err := drainReadyAllocations(results, func(allocationResult) error { consumed++; return nil })
	if err != nil || consumed != 8 {
		t.Fatalf("consumed=%d err=%v", consumed, err)
	}
	if err := drainReadyAllocations(results, func(allocationResult) error { t.Fatal("empty channel consumed"); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestDrainReadyAllocationsStopsBeforeLaunchingOnFailure(t *testing.T) {
	failure := errors.New("allocation failed")
	results := make(chan allocationResult, 2)
	results <- allocationResult{err: failure}
	results <- allocationResult{activityID: "later"}
	consumed := 0
	err := drainReadyAllocations(results, func(r allocationResult) error { consumed++; return r.err })
	if !errors.Is(err, failure) || consumed != 1 || len(results) != 1 {
		t.Fatalf("consumed=%d err=%v remaining=%d", consumed, err, len(results))
	}
}
