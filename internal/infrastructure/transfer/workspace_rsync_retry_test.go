package transfer

import (
	"context"
	"errors"
	"testing"
)

func TestWorkspaceRsyncRetriesTransientFailureAndAccountsReportedBytes(t *testing.T) {
	calls := 0
	stats, err := retryWorkspaceRsync(context.Background(), 0, false, func() ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("rsync: connection unexpectedly closed (100 bytes received so far) [receiver]\nBroken pipe"), errors.New("exit 255")
		}
		return []byte("Total bytes received: 50\nNumber of regular files transferred: 1\nTotal transferred file size: 40"), nil
	})
	if err != nil || calls != 2 || stats.bytes != 150 || stats.retries != 1 || !stats.networkBytesIncomplete || stats.lastRetryError == "" {
		t.Fatalf("retry statistics: calls=%d stats=%+v err=%v", calls, stats, err)
	}
}

func TestWorkspaceRsyncDoesNotRetryPermanentErrorsOrCancellation(t *testing.T) {
	for _, message := range []string{"Permission denied\nBroken pipe", "workspace content conflict", "No space left on device", "No such file or directory"} {
		calls := 0
		_, err := retryWorkspaceRsync(context.Background(), 0, false, func() ([]byte, error) {
			calls++
			return []byte(message), errors.New("copy failed")
		})
		if err == nil || calls != 1 {
			t.Fatalf("permanent error retried: %q calls=%d", message, calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := retryWorkspaceRsync(ctx, 0, false, func() ([]byte, error) {
		t.Fatal("copy executed after cancellation")
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestWorkspaceRsyncTransientRetriesAreBounded(t *testing.T) {
	calls := 0
	_, err := retryWorkspaceRsync(context.Background(), 0, false, func() ([]byte, error) {
		calls++
		return []byte("connection reset by peer"), errors.New("exit 255")
	})
	if err == nil || calls != 3 {
		t.Fatalf("unbounded or missing retries: calls=%d err=%v", calls, err)
	}
}
