package gcp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHostKeyWaitRetriesOnlyPublicationPending(t *testing.T) {
	calls := 0
	result, err := waitForHostKey(context.Background(), time.Millisecond, func(context.Context) (HostIdentity, error) {
		calls++
		if calls == 1 {
			return HostIdentity{}, errHostKeyPublicationPending
		}
		return HostIdentity{Generation: "123"}, nil
	})
	if err != nil || calls != 2 || result.Generation != "123" {
		t.Fatalf("calls=%d result=%+v err=%v", calls, result, err)
	}
	calls = 0
	_, err = waitForHostKey(context.Background(), time.Millisecond, func(context.Context) (HostIdentity, error) {
		calls++
		return HostIdentity{}, errors.New("generation mismatch")
	})
	if err == nil || calls != 1 {
		t.Fatal("retried identity mismatch")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = waitForHostKey(ctx, time.Hour, func(context.Context) (HostIdentity, error) { return HostIdentity{}, errHostKeyPublicationPending })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
