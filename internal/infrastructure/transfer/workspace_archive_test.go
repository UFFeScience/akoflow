package transfer

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestArchivePreparationRunsTwoWorkersAndWaitsForBarrier(t *testing.T) {
	entered := make(chan int, 4)
	release := make(chan struct{})
	done := make(chan error, 1)
	var active, peak, completed atomic.Int32
	go func() {
		done <- parallelWorkspacePreparation(context.Background(), 4, 2, func(ctx context.Context, i int) error {
			n := active.Add(1)
			for old := peak.Load(); n > old; old = peak.Load() {
				if peak.CompareAndSwap(old, n) {
					break
				}
			}
			defer active.Add(-1)
			entered <- i
			<-release
			completed.Add(1)
			return nil
		})
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("preparations did not overlap")
		}
	}
	select {
	case <-entered:
		t.Fatal("parallelism exceeded two")
	case <-done:
		t.Fatal("barrier released early")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 2 || completed.Load() != 4 {
		t.Fatalf("peak=%d completed=%d", peak.Load(), completed.Load())
	}
}

func TestArchivePreparationCancelsAndJoinsWorkersOnFailure(t *testing.T) {
	started := make(chan struct{})
	var stopped atomic.Bool
	err := parallelWorkspacePreparation(context.Background(), 2, 2, func(ctx context.Context, i int) error {
		if i == 0 {
			<-started
			return fmt.Errorf("injected failure")
		}
		close(started)
		<-ctx.Done()
		stopped.Store(true)
		return ctx.Err()
	})
	if err == nil || !stopped.Load() {
		t.Fatalf("failed barrier returned before worker stopped: %v", err)
	}
}

func TestWorkspaceArchiveRejectsUnsafeMembers(t *testing.T) {
	for _, test := range []struct {
		name string
		kind byte
		link string
	}{
		{"../escape", tar.TypeReg, ""}, {"/absolute", tar.TypeReg, ""},
		{"link", tar.TypeSymlink, "/outside"}, {"hardlink", tar.TypeLink, "other"},
		{"fifo", tar.TypeFifo, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "bad.tar")
			file, err := os.Create(filename)
			if err != nil {
				t.Fatal(err)
			}
			writer := tar.NewWriter(file)
			if err := writer.WriteHeader(&tar.Header{Name: test.name, Typeflag: test.kind, Linkname: test.link, Mode: 0600}); err != nil {
				t.Fatal(err)
			}
			writer.Close()
			file.Close()
			if _, err := validateWorkspaceArchive(filename); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestUncompressedWorkspaceArchiveRoundtripDigest(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "input.fits"), []byte("unchanged content"), 0600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "workspace.tar")
	if err := packLocalWorkspaceArchive(context.Background(), root, filename); err != nil {
		t.Fatal(err)
	}
	digest, err := validateWorkspaceArchive(filename)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if digest != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal("archive digest does not cover full transmitted file")
	}
	if len(data) < 1024 {
		t.Fatal("not an uncompressed tar archive")
	}
}
