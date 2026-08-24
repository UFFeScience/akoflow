package filesystem

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Driver struct {
	typeName domain.StorageType
	root     string
}

func New(typeName domain.StorageType, root string) (*Driver, error) {
	if typeName != domain.StorageLocal && typeName != domain.StoragePVC &&
		typeName != domain.StorageNFS && typeName != domain.StorageLustre {
		return nil, fmt.Errorf("filesystem storage type %q is unsupported", typeName)
	}
	absolute, err := filepath.Abs(root)
	if err != nil || strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("filesystem storage root is required")
	}
	return &Driver{typeName: typeName, root: filepath.Clean(absolute)}, nil
}

func (d *Driver) Type() domain.StorageType { return d.typeName }

// List deliberately reads one directory only. This makes browsing large HPC
// filesystems predictable and prevents discovery from becoming a global scan.
func (d *Driver) Browse(_ context.Context, storage domain.StorageResource, request domain.BrowseRequest) (domain.BrowsePage, error) {
	directory, relative, err := d.browsePath(storage, request.Path)
	if err != nil {
		return domain.BrowsePage{}, err
	}
	items, err := os.ReadDir(directory)
	if err != nil {
		return domain.BrowsePage{}, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })
	start := 0
	if request.Cursor != "" {
		start, err = strconv.Atoi(request.Cursor)
		if err != nil || start < 0 {
			return domain.BrowsePage{}, fmt.Errorf("invalid browse cursor")
		}
	}
	limit := request.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	if start > len(items) {
		start = len(items)
	}
	entries := make([]domain.FileEntry, 0, end-start)
	for _, item := range items[start:end] {
		full := filepath.Join(directory, item.Name())
		info, infoErr := os.Lstat(full)
		if infoErr != nil {
			return domain.BrowsePage{}, infoErr
		}
		entry, err := d.infoEntry(storage.ID, filepath.ToSlash(filepath.Join(relative, item.Name())), info, full)
		if err != nil {
			return domain.BrowsePage{}, err
		}
		entries = append(entries, entry)
	}
	page := domain.BrowsePage{StorageID: storage.ID, Path: relative, Entries: entries}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (d *Driver) BrowseStat(_ context.Context, storage domain.StorageResource, value string) (domain.FileEntry, error) {
	full, relative, err := d.browsePath(storage, value)
	if err != nil {
		return domain.FileEntry{}, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return d.infoEntry(storage.ID, relative, info, full)
}

func (d *Driver) Open(_ context.Context, storage domain.StorageResource, value string) (io.ReadCloser, domain.FileEntry, error) {
	entry, err := d.BrowseStat(context.Background(), storage, value)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	if entry.Type != domain.FileEntryFile {
		return nil, domain.FileEntry{}, fmt.Errorf("only files can be downloaded")
	}
	full, _, err := d.browsePath(storage, value)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	f, err := os.Open(full)
	return f, entry, err
}
func (d *Driver) Remove(_ context.Context, storage domain.StorageResource, value string) error {
	if storage.ReadOnly {
		return fmt.Errorf("storage is read-only")
	}
	full, _, err := d.browsePath(storage, value)
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("directories require an explicit archive/cleanup job")
	}
	return os.Remove(full)
}
func (d *Driver) Write(
	_ context.Context,
	storage domain.StorageResource,
	value string,
	source io.Reader,
	_ int64,
) error {
	if storage.ReadOnly {
		return fmt.Errorf("storage is read-only")
	}
	root, err := d.storageRoot(storage)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0o750); err != nil {
		return err
	}
	full, _, err := d.browsePath(storage, value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	tmp := full + ".akoflow-copy-part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, source)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		return firstError(copyErr, closeErr)
	}
	return os.Rename(tmp, full)
}

func (d *Driver) browsePath(storage domain.StorageResource, value string) (string, string, error) {
	root, err := d.storageRoot(storage)
	if err != nil {
		return "", "", err
	}
	relative := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(value, "/")))
	if relative == "." {
		relative = ""
	}
	if strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return "", "", fmt.Errorf("browse path escapes its root")
	}
	full := filepath.Join(root, relative)
	// Evaluate the closest existing parent. This permits the first write into a
	// new storage while still preventing an existing symlink from escaping.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	existing := full
	for {
		if _, statErr := os.Lstat(existing); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", "", statErr
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", "", fmt.Errorf("no existing parent for browse path")
		}
		existing = parent
	}
	realPath, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", "", err
	}
	if realPath != realRoot && !strings.HasPrefix(realPath, realRoot+string(filepath.Separator)) {
		return "", "", fmt.Errorf("browse path escapes via symlink")
	}
	return full, filepath.ToSlash(relative), nil
}

func (d *Driver) storageRoot(storage domain.StorageResource) (string, error) {
	root := d.root
	if storage.Endpoint != "" {
		candidate, err := filepath.Abs(storage.Endpoint)
		if err != nil {
			return "", err
		}
		root = filepath.Clean(candidate)
	}
	allowed := false
	for _, configured := range storage.BrowseRoots {
		candidate, err := filepath.Abs(configured.Path)
		if err == nil && filepath.Clean(candidate) == root {
			allowed = true
			break
		}
	}
	if len(storage.BrowseRoots) > 0 && !allowed {
		return "", fmt.Errorf("storage endpoint is not an allowed browse root")
	}
	return root, nil
}

func (d *Driver) entry(storageID, parent string, item os.DirEntry) (domain.FileEntry, error) {
	full := filepath.Join(d.root, parent, item.Name())
	info, err := os.Lstat(full)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return d.infoEntry(storageID, filepath.ToSlash(filepath.Join(parent, item.Name())), info, full)
}
func (d *Driver) infoEntry(storageID, relative string, info os.FileInfo, full string) (domain.FileEntry, error) {
	typ := domain.FileEntryFile
	if info.IsDir() {
		typ = domain.FileEntryDirectory
	}
	if info.Mode()&os.ModeSymlink != 0 {
		typ = domain.FileEntrySymlink
		target, _ := os.Readlink(full)
		return domain.FileEntry{StorageID: storageID, Path: relative, Name: filepath.Base(relative), Type: typ, ModifiedAt: info.ModTime(), Readable: true, LinkTarget: target}, nil
	}
	return domain.FileEntry{StorageID: storageID, Path: relative, Name: filepath.Base(relative), Type: typ, SizeBytes: info.Size(), ModifiedAt: info.ModTime(), Readable: true}, nil
}

func (d *Driver) Put(_ context.Context, request ports.PutObjectRequest) (domain.DataLocation, error) {
	destination, err := d.resolve(request.Key)
	if err != nil {
		return domain.DataLocation{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return domain.DataLocation{}, err
	}
	temporary := destination + ".akoflow-part"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return domain.DataLocation{}, err
	}
	_, copyErr := io.Copy(file, request.Source)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(temporary)
		return domain.DataLocation{}, fmt.Errorf("write storage object: %w", firstError(copyErr, closeErr))
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return domain.DataLocation{}, err
	}
	return domain.DataLocation{URI: (&url.URL{Scheme: "file", Path: destination}).String(),
		Status: domain.DataLocationAvailable}, nil
}

func (d *Driver) Get(_ context.Context, request ports.GetObjectRequest) error {
	path, err := filePath(request.Location.URI)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(request.Target, file)
	return err
}

func (d *Driver) Stat(_ context.Context, location domain.DataLocation) (ports.ObjectStat, error) {
	path, err := filePath(location.URI)
	if err != nil {
		return ports.ObjectStat{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return ports.ObjectStat{}, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return ports.ObjectStat{}, err
	}
	return ports.ObjectStat{SizeBytes: size, Checksum: fmt.Sprintf("sha256:%x", hash.Sum(nil))}, nil
}

func (d *Driver) Delete(_ context.Context, location domain.DataLocation) error {
	path, err := filePath(location.URI)
	if err != nil {
		return err
	}
	resolved, err := d.resolve(strings.TrimPrefix(path, d.root+string(filepath.Separator)))
	if err != nil || resolved != filepath.Clean(path) {
		return fmt.Errorf("location is outside storage root")
	}
	return os.Remove(path)
}

func (d *Driver) resolve(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage key %q escapes its root", key)
	}
	return filepath.Join(d.root, clean), nil
}

func filePath(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "file" {
		return "", fmt.Errorf("file location URI is required")
	}
	return filepath.Clean(parsed.Path), nil
}

func firstError(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
