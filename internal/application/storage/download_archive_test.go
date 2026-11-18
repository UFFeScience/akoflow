package storage

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestQueuedRemoteArchiveCanBeDownloadedWithExpectedFiles(t *testing.T) {
	ctx := context.Background()
	service, catalog, browser := coordinatorFixture()
	catalog.storages["remote"] = storageFixture("remote", domain.StorageSSH)
	service.resolver = Registry{domain.StorageSSH: browser}
	directory := domain.FileEntry{Path: "/data/input", Type: domain.FileEntryDirectory}
	file := fileFixture("/data/input/sample.txt", "remote contents")
	browser.entries[directory.Path] = directory
	browser.entries[file.Path] = file
	browser.contents[file.Path] = "remote contents"
	browser.children[directory.Path] = []domain.FileEntry{file}
	run, err := service.QueueArchive(ctx, "remote", directory.Path, "queued-archive")
	if err != nil || run.Status != domain.DownloadQueued {
		t.Fatalf("queue remote archive: %+v, %v", run, err)
	}
	ready := waitDownload(t, catalog, run.ID, domain.DownloadReady)
	browser.mu.Lock()
	browser.entries[ready.Path] = fileFixture(ready.Path, browser.contents[ready.Path])
	browser.mu.Unlock()
	body, _, err := service.OpenDownload(ctx, ready.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	assertArchive(t, string(data), map[string]string{"data/input/sample.txt": "remote contents"})

	browser.writeErr = errors.New("remote disk is full")
	failed, err := service.QueueArchive(ctx, "remote", directory.Path, "failed-archive")
	if err != nil {
		t.Fatal(err)
	}
	waitDownload(t, catalog, failed.ID, domain.DownloadFailed)
	if stream, _, err := service.OpenDownload(ctx, failed.ID); err == nil {
		stream.Close()
		t.Fatal("failed archive was available to download")
	}
}
