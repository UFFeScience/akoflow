package storage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type catalogStub struct {
	mu          sync.Mutex
	storages    map[string]domain.StorageResource
	downloads   map[string]domain.DownloadRun
	indexRuns   []domain.IndexRun
	promoted    int
	artifacts   int
	listErr     error
	saveErr     error
	findDownErr error
}

func (c *catalogStub) ListEnvironmentStorages(context.Context, string) ([]domain.StorageResource, error) {
	if c.listErr != nil { return nil, c.listErr }
	c.mu.Lock(); defer c.mu.Unlock()
	result := make([]domain.StorageResource, 0, len(c.storages))
	for _, value := range c.storages { result = append(result, value) }
	return result, nil
}
func (c *catalogStub) FindStorage(_ context.Context, id string) (*domain.StorageResource, error) {
	c.mu.Lock(); defer c.mu.Unlock()
	v, ok := c.storages[id]
	if !ok { return nil, nil }
	return &v, nil
}
func (c *catalogStub) SaveDownload(_ context.Context, run domain.DownloadRun) error {
	if c.saveErr != nil { return c.saveErr }
	c.mu.Lock(); defer c.mu.Unlock()
	if c.downloads == nil { c.downloads = map[string]domain.DownloadRun{} }
	c.downloads[run.ID] = run
	return nil
}
func (c *catalogStub) FindDownload(_ context.Context, id string) (*domain.DownloadRun, error) {
	if c.findDownErr != nil { return nil, c.findDownErr }
	c.mu.Lock(); defer c.mu.Unlock()
	v, ok := c.downloads[id]
	if !ok { return nil, nil }
	return &v, nil
}
func (c *catalogStub) PromoteData(context.Context, string, string, string, string, string, string, domain.FileEntry, string) error {
	c.mu.Lock(); defer c.mu.Unlock(); c.promoted++; return nil
}
func (c *catalogStub) PromoteArtifact(context.Context, string, string, string, string, string, string, string, string, domain.FileEntry) error {
	c.mu.Lock(); defer c.mu.Unlock(); c.artifacts++; return nil
}
func (c *catalogStub) SaveIndexRun(_ context.Context, run domain.IndexRun) error {
	c.mu.Lock(); defer c.mu.Unlock(); c.indexRuns = append(c.indexRuns, run); return nil
}
func (c *catalogStub) ListIndexRuns(context.Context, string) ([]domain.IndexRun, error) {
	c.mu.Lock(); defer c.mu.Unlock(); return append([]domain.IndexRun(nil), c.indexRuns...), nil
}
func (c *catalogStub) download(id string) domain.DownloadRun {
	c.mu.Lock(); defer c.mu.Unlock(); return c.downloads[id]
}

type browserStub struct {
	mu        sync.Mutex
	entries   map[string]domain.FileEntry
	children  map[string][]domain.FileEntry
	contents  map[string]string
	removed   []string
	browseErr error
	openErr   error
	writeErr  error
}

func (b *browserStub) Browse(_ context.Context, storage domain.StorageResource, r domain.BrowseRequest) (domain.BrowsePage, error) {
	if b.browseErr != nil { return domain.BrowsePage{}, b.browseErr }
	b.mu.Lock(); defer b.mu.Unlock()
	return domain.BrowsePage{StorageID: storage.ID, Path: r.Path, Entries: append([]domain.FileEntry(nil), b.children[r.Path]...)}, nil
}
func (b *browserStub) BrowseStat(_ context.Context, _ domain.StorageResource, path string) (domain.FileEntry, error) {
	b.mu.Lock(); defer b.mu.Unlock()
	v, ok := b.entries[path]
	if !ok { return domain.FileEntry{}, fmt.Errorf("missing %s", path) }
	return v, nil
}
func (b *browserStub) Open(_ context.Context, _ domain.StorageResource, path string) (io.ReadCloser, domain.FileEntry, error) {
	if b.openErr != nil { return nil, domain.FileEntry{}, b.openErr }
	b.mu.Lock(); defer b.mu.Unlock()
	v, ok := b.entries[path]
	if !ok { return nil, domain.FileEntry{}, fmt.Errorf("missing %s", path) }
	return io.NopCloser(strings.NewReader(b.contents[path])), v, nil
}
func (b *browserStub) Remove(_ context.Context, _ domain.StorageResource, path string) error {
	b.mu.Lock(); defer b.mu.Unlock(); b.removed = append(b.removed, path); return nil
}
func (b *browserStub) Write(_ context.Context, _ domain.StorageResource, path string, body io.Reader, _ int64) error {
	if b.writeErr != nil { return b.writeErr }
	data, err := io.ReadAll(body)
	if err != nil { return err }
	b.mu.Lock(); defer b.mu.Unlock()
	if b.contents == nil { b.contents = map[string]string{} }
	b.contents[path] = string(data)
	return nil
}
func (b *browserStub) content(path string) string { b.mu.Lock(); defer b.mu.Unlock(); return b.contents[path] }

func storageFixture(id string, typ domain.StorageType) domain.StorageResource {
	return domain.StorageResource{ID: id, Type: typ, BrowseRoots: []domain.StorageBrowseRoot{{Path: "/data"}}, Health: domain.StorageHealthStatus{Status: domain.StorageHealth("healthy")}}
}
func fileFixture(path, value string) domain.FileEntry {
	return domain.FileEntry{Path: path, Name: strings.TrimPrefix(path, "/data/"), Type: domain.FileEntryFile, SizeBytes: int64(len(value)), ModifiedAt: time.Unix(10, 0), Readable: true, Writable: true}
}
func coordinatorFixture() (*BrowserCoordinator, *catalogStub, *browserStub) {
	storage := storageFixture("source", domain.StorageLocal)
	destination := storageFixture("destination", domain.StorageLocal)
	browser := &browserStub{entries: map[string]domain.FileEntry{}, children: map[string][]domain.FileEntry{}, contents: map[string]string{}}
	catalog := &catalogStub{storages: map[string]domain.StorageResource{storage.ID: storage, destination.ID: destination}, downloads: map[string]domain.DownloadRun{}}
	return NewBrowserCoordinator(catalog, Registry{domain.StorageLocal: browser}), catalog, browser
}

func TestListRootsBrowseStatAndDelete(t *testing.T) {
	s, catalog, browser := coordinatorFixture()
	file := fileFixture("/data/a.txt", "hello")
	browser.entries[file.Path], browser.children["/data"], browser.contents[file.Path] = file, []domain.FileEntry{file}, "hello"
	readOnly := storageFixture("readonly", domain.StorageS3); readOnly.ReadOnly = true; readOnly.Shared = true
	missingDriver := storageFixture("missing-driver", domain.StorageGCS)
	catalog.storages[readOnly.ID], catalog.storages[missingDriver.ID] = readOnly, missingDriver
	s.resolver = Registry{domain.StorageLocal: browser, domain.StorageS3: browser}

	values, err := s.List(context.Background(), "environment")
	if err != nil || len(values) != 4 { t.Fatalf("List() = %d, %v", len(values), err) }
	byID := map[string]domain.StorageResource{}
	for _, value := range values { byID[value.ID] = value }
	if !byID["source"].Capabilities.Write || byID["readonly"].Capabilities.Write || !byID["readonly"].Capabilities.PresignedDownload { t.Fatalf("unexpected capabilities: %#v", byID) }
	if byID["missing-driver"].Health.Status != domain.StorageHealth("unavailable") { t.Fatalf("health = %s", byID["missing-driver"].Health.Status) }
	roots, err := s.Roots(context.Background(), "source")
	if err != nil || len(roots) != 1 { t.Fatalf("Roots() = %#v, %v", roots, err) }
	page, err := s.Browse(context.Background(), "source", domain.BrowseRequest{Path: "/data"})
	if err != nil || len(page.Entries) != 1 { t.Fatalf("Browse() = %#v, %v", page, err) }
	entry, err := s.Stat(context.Background(), "source", file.Path)
	if err != nil || entry.Path != file.Path { t.Fatalf("Stat() = %#v, %v", entry, err) }
	if err = s.Delete(context.Background(), "source", file.Path); err != nil { t.Fatal(err) }
	if len(browser.removed) != 1 { t.Fatalf("removed = %#v", browser.removed) }
	if err = s.Delete(context.Background(), "readonly", file.Path); err == nil || !strings.Contains(err.Error(), "read-only") { t.Fatalf("Delete(readonly) = %v", err) }
}

func TestDownloadsChecksumsAndPromotions(t *testing.T) {
	s, catalog, browser := coordinatorFixture()
	content := "scientific payload"
	file := fileFixture("/data/result.txt", content)
	image := fileFixture("/data/tool.SIF", content)
	directory := domain.FileEntry{Path: "/data/dir", Type: domain.FileEntryDirectory}
	for _, entry := range []domain.FileEntry{file, image, directory} { browser.entries[entry.Path] = entry }
	browser.contents[file.Path], browser.contents[image.Path] = content, content

	run, err := s.StartDownload(context.Background(), "source", file.Path, "download-1")
	if err != nil || run.Status != domain.DownloadReady { t.Fatalf("StartDownload() = %#v, %v", run, err) }
	body, entry, err := s.OpenDownload(context.Background(), run.ID)
	if err != nil || entry.Path != file.Path { t.Fatalf("OpenDownload() = %#v, %v", entry, err) }
	data, _ := io.ReadAll(body); _ = body.Close()
	if string(data) != content { t.Fatalf("content = %q", data) }
	stored, err := s.Download(context.Background(), run.ID)
	if err != nil || stored.ID != run.ID { t.Fatalf("Download() = %#v, %v", stored, err) }
	sum, err := s.Checksum(context.Background(), "source", file.Path)
	want := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
	if err != nil || sum != want { t.Fatalf("Checksum() = %q, %v; want %q", sum, err, want) }
	if err = s.PromoteData(context.Background(), "source", file.Path, "wv", "run", "activity", "data"); err != nil { t.Fatal(err) }
	if err = s.PromoteArtifact(context.Background(), "source", image.Path, "artifact", "tool", "1", "global", ""); err != nil { t.Fatal(err) }
	if catalog.promoted != 1 || catalog.artifacts != 1 { t.Fatalf("promotions = %d/%d", catalog.promoted, catalog.artifacts) }
	if _, err = s.StartDownload(context.Background(), "source", directory.Path, "bad"); err == nil { t.Fatal("expected directory download error") }
	if _, err = s.Checksum(context.Background(), "source", directory.Path); err == nil { t.Fatal("expected directory checksum error") }
	if err = s.PromoteData(context.Background(), "source", directory.Path, "", "", "", ""); err == nil { t.Fatal("expected directory promotion error") }
	if err = s.PromoteArtifact(context.Background(), "source", file.Path, "", "", "", "", ""); err == nil { t.Fatal("expected extension error") }
}

func TestIndexWalksRootsWithFiltersAndLimits(t *testing.T) {
	s, catalog, browser := coordinatorFixture()
	storage := catalog.storages["source"]
	storage.IndexPolicy = domain.IndexPolicy{Enabled: true, Roots: []string{"/data"}, MaxDepth: 3, MaxEntries: 10, Include: []string{"*.txt"}, Exclude: []string{"skip*"}}
	catalog.storages["source"] = storage
	dir := domain.FileEntry{Path: "/data/nested", Type: domain.FileEntryDirectory}
	keep := fileFixture("/data/nested/keep.txt", "yes")
	skip := fileFixture("/data/nested/skip.txt", "no")
	ignored := fileFixture("/data/image.png", "png")
	browser.children["/data"] = []domain.FileEntry{dir, ignored}
	browser.children[dir.Path] = []domain.FileEntry{keep, skip}

	run, err := s.StartIndex(context.Background(), "source", "index-1")
	if err != nil || run.Status != "completed" || run.IndexedEntries != 2 { t.Fatalf("StartIndex() = %#v, %v", run, err) }
	runs, err := s.IndexRuns(context.Background(), "source")
	if err != nil || len(runs) != 2 || runs[1].Status != "completed" { t.Fatalf("IndexRuns() = %#v, %v", runs, err) }
	storage.IndexPolicy.Enabled = false; catalog.storages["source"] = storage
	if _, err = s.StartIndex(context.Background(), "source", "disabled"); err == nil { t.Fatal("expected disabled error") }
	browser.browseErr = fmt.Errorf("browse failed")
	storage.IndexPolicy.Enabled = true; catalog.storages["source"] = storage
	run, err = s.StartIndex(context.Background(), "source", "failed")
	if err == nil || run.Status != "failed" { t.Fatalf("failed index = %#v, %v", run, err) }
}

func TestCopyAndArchiveJobsComplete(t *testing.T) {
	s, catalog, browser := coordinatorFixture()
	content := "large result"
	file := fileFixture("/data/result.txt", content)
	browser.entries[file.Path], browser.contents[file.Path] = file, content

	run, err := s.QueueCopy(context.Background(), "source", file.Path, "destination", "copy-1")
	if err != nil || run.Status != domain.DownloadQueued { t.Fatalf("QueueCopy() = %#v, %v", run, err) }
	waitDownload(t, catalog, run.ID, domain.DownloadCompleted)
	if got := browser.content(file.Path); got != content { t.Fatalf("copied content = %q", got) }

	directory := domain.FileEntry{Path: "/data/tree", Type: domain.FileEntryDirectory}
	nested := domain.FileEntry{Path: "/data/tree/nested", Type: domain.FileEntryDirectory}
	a := fileFixture("/data/tree/a.txt", "alpha")
	b := fileFixture("/data/tree/nested/b.txt", "beta")
	symlink := domain.FileEntry{Path: "/data/tree/link", Type: domain.FileEntrySymlink}
	browser.entries[directory.Path], browser.entries[a.Path], browser.entries[b.Path] = directory, a, b
	browser.contents[a.Path], browser.contents[b.Path] = "alpha", "beta"
	browser.children[directory.Path] = []domain.FileEntry{a, nested, symlink}
	browser.children[nested.Path] = []domain.FileEntry{b}
	run, err = s.QueueArchive(context.Background(), "source", directory.Path, "archive-1")
	if err != nil { t.Fatal(err) }
	completed := waitDownload(t, catalog, run.ID, domain.DownloadReady)
	if completed.Path != directory.Path+".tar.gz" { t.Fatalf("archive path = %q", completed.Path) }
	assertArchive(t, browser.content(completed.Path), map[string]string{"data/tree/a.txt": "alpha", "data/tree/nested/b.txt": "beta"})
	if _, err = s.QueueArchive(context.Background(), "source", file.Path, "bad-archive"); err == nil { t.Fatal("expected file archive error") }
}

func TestCoordinatorRejectsUnavailableResourcesAndFailedJobs(t *testing.T) {
	s, catalog, browser := coordinatorFixture()
	file := fileFixture("/data/file", "x"); browser.entries[file.Path], browser.contents[file.Path] = file, "x"
	if _, err := s.Roots(context.Background(), "unknown"); err == nil { t.Fatal("expected missing storage") }
	if _, err := s.Download(context.Background(), "unknown"); err == nil { t.Fatal("expected missing download") }
	if _, _, err := s.OpenDownload(context.Background(), "unknown"); err == nil { t.Fatal("expected missing download") }
	noRoots := storageFixture("no-roots", domain.StorageLocal); noRoots.BrowseRoots = nil; catalog.storages[noRoots.ID] = noRoots
	if _, err := s.Browse(context.Background(), noRoots.ID, domain.BrowseRequest{}); err == nil { t.Fatal("expected roots error") }
	unavailable := storageFixture("offline", domain.StorageLocal); unavailable.Health = domain.StorageHealthStatus{Status: domain.StorageHealth("unavailable"), Message: "network"}; catalog.storages[unavailable.ID] = unavailable
	if _, err := s.Stat(context.Background(), unavailable.ID, file.Path); err == nil { t.Fatal("expected unavailable error") }
	if _, err := (Registry{}).Browser(domain.StorageSSH); err == nil { t.Fatal("expected missing driver") }
	if _, err := s.QueueCopy(context.Background(), "source", file.Path, "unknown", "failed-copy"); err == nil { t.Fatal("expected missing destination") }
	if got := catalog.download("failed-copy"); got.Status != domain.DownloadFailed { t.Fatalf("failed copy = %#v", got) }
	browser.writeErr = fmt.Errorf("disk full")
	run, err := s.QueueCopy(context.Background(), "source", file.Path, "destination", "async-failure")
	if err != nil { t.Fatal(err) }
	waitDownload(t, catalog, run.ID, domain.DownloadFailed)
}

func waitDownload(t *testing.T, catalog *catalogStub, id string, status domain.DownloadStatus) domain.DownloadRun {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run := catalog.download(id)
		if run.Status == status { return run }
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("download %s did not reach %s: %#v", id, status, catalog.download(id))
	return domain.DownloadRun{}
}

func assertArchive(t *testing.T, raw string, want map[string]string) {
	t.Helper()
	gz, err := gzip.NewReader(strings.NewReader(raw)); if err != nil { t.Fatal(err) }; defer gz.Close()
	tr := tar.NewReader(gz)
	got := map[string]string{}
	for { h, err := tr.Next(); if err == io.EOF { break }; if err != nil { t.Fatal(err) }; data, _ := io.ReadAll(tr); got[h.Name] = string(data) }
	if len(got) != len(want) { t.Fatalf("archive files = %#v", got) }
	for name, value := range want { if got[name] != value { t.Fatalf("archive[%s] = %q", name, got[name]) } }
}
