package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
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

func TestWorkspaceRsyncRetryReusesRealChecksumValidFiles(t *testing.T) {
	realRsync, err := exec.LookPath("rsync")
	if err != nil {
		t.Skip("rsync unavailable")
	}
	root := t.TempDir()
	source, destination, bin := filepath.Join(root, "source"), filepath.Join(root, "destination"), filepath.Join(root, "bin")
	for _, directory := range []string{source, destination, bin} {
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "complete.fits"), []byte("checksum-valid producer data"), 0600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "interrupted")
	script := fmt.Sprintf("#!/bin/sh\nif test ! -e %s; then %s \"$@\" || exit $?; touch %s; echo 'connection unexpectedly closed (100 bytes received so far)' >&2; exit 255; fi\nexec %s \"$@\"\n", shell(marker), shell(realRsync), shell(marker), shell(realRsync))
	if err := os.WriteFile(filepath.Join(bin, "rsync"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	stats, err := syncDirectory(context.Background(), domain.TransferEndpoint{URI: workspaceURL(source)}, domain.TransferEndpoint{URI: workspaceURL(destination)}, false)
	if err != nil || stats.retries != 1 || stats.files != 0 {
		t.Fatalf("validated files recopied on retry: stats=%+v err=%v", stats, err)
	}
	for _, directory := range []string{source, destination} {
		content, err := os.ReadFile(filepath.Join(directory, "complete.fits"))
		if err != nil || string(content) != "checksum-valid producer data" {
			t.Fatalf("producer or destination corrupted: %q err=%v", content, err)
		}
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
