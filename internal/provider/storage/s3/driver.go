package s3

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type CredentialResolver interface {
	Authorize(context.Context, *http.Request, string) error
}

type Driver struct {
	client      *http.Client
	credentials CredentialResolver
}

func New(client *http.Client, credentials CredentialResolver) *Driver {
	if client == nil {
		client = http.DefaultClient
	}
	return &Driver{client: client, credentials: credentials}
}

func (*Driver) Type() domain.StorageType { return domain.StorageS3 }

// List maps the S3 ListObjectsV2 prefix/delimiter API to the same virtual
// directory contract used by POSIX storage. MinIO and most S3-compatible
// gateways support this API as well.
func (d *Driver) Browse(ctx context.Context, storage domain.StorageResource, browse domain.BrowseRequest) (domain.BrowsePage, error) {
	prefix, err := objectPrefix(browse.Path)
	if err != nil {
		return domain.BrowsePage{}, err
	}
	endpoint := strings.TrimRight(storage.Endpoint, "/") + "?list-type=2&delimiter=/&prefix=" + url.QueryEscape(prefix)
	if browse.Cursor != "" {
		endpoint += "&continuation-token=" + url.QueryEscape(browse.Cursor)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.BrowsePage{}, err
	}
	if err = d.authorize(ctx, req, storage.CredentialReference); err != nil {
		return domain.BrowsePage{}, err
	}
	response, err := d.client.Do(req)
	if err != nil {
		return domain.BrowsePage{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.BrowsePage{}, fmt.Errorf("S3 list returned %s", response.Status)
	}
	var result struct {
		Contents []struct {
			Key          string `xml:"Key"`
			Size         int64  `xml:"Size"`
			ETag         string `xml:"ETag"`
			LastModified string `xml:"LastModified"`
		} `xml:"Contents"`
		Prefixes []struct {
			Prefix string `xml:"Prefix"`
		} `xml:"CommonPrefixes"`
		Next      string `xml:"NextContinuationToken"`
		Truncated bool   `xml:"IsTruncated"`
	}
	if err := xml.NewDecoder(response.Body).Decode(&result); err != nil {
		return domain.BrowsePage{}, err
	}
	entries := make([]domain.FileEntry, 0, len(result.Contents)+len(result.Prefixes))
	for _, p := range result.Prefixes {
		name := strings.TrimSuffix(strings.TrimPrefix(p.Prefix, prefix), "/")
		entries = append(entries, domain.FileEntry{StorageID: storage.ID, Path: strings.TrimSuffix(p.Prefix, "/"), Name: name, Type: domain.FileEntryDirectory, Readable: true})
	}
	for _, o := range result.Contents {
		if o.Key == prefix {
			continue
		}
		entries = append(entries, domain.FileEntry{StorageID: storage.ID, Path: o.Key, Name: path.Base(o.Key), Type: domain.FileEntryFile, SizeBytes: o.Size, ETag: strings.Trim(o.ETag, "\""), Readable: true})
	}
	return domain.BrowsePage{StorageID: storage.ID, Path: strings.TrimSuffix(prefix, "/"), Entries: entries, NextCursor: result.Next}, nil
}

func (d *Driver) Open(ctx context.Context, storage domain.StorageResource, key string) (io.ReadCloser, domain.FileEntry, error) {
	entry, err := d.StatEntry(ctx, storage, key)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	u, err := objectURL(storage.Endpoint, key)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	if err = d.authorize(ctx, req, storage.CredentialReference); err != nil {
		return nil, domain.FileEntry{}, err
	}
	response, err := d.client.Do(req)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, domain.FileEntry{}, fmt.Errorf("S3 get returned %s", response.Status)
	}
	return response.Body, entry, nil
}

func (d *Driver) BrowseStat(ctx context.Context, storage domain.StorageResource, key string) (domain.FileEntry, error) {
	return d.StatEntry(ctx, storage, key)
}
func (d *Driver) Remove(ctx context.Context, storage domain.StorageResource, key string) error {
	if storage.ReadOnly {
		return fmt.Errorf("storage is read-only")
	}
	u, err := objectURL(storage.Endpoint, key)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	if err = d.authorize(ctx, req, storage.CredentialReference); err != nil {
		return err
	}
	return d.do(req, nil)
}
func (d *Driver) Write(ctx context.Context, storage domain.StorageResource, key string, source io.Reader, size int64) error {
	_, err := d.Put(ctx, ports.PutObjectRequest{Storage: storage, Key: key, Source: source, Size: size})
	return err
}
func (d *Driver) StatEntry(ctx context.Context, storage domain.StorageResource, key string) (domain.FileEntry, error) {
	u, err := objectURL(storage.Endpoint, key)
	if err != nil {
		return domain.FileEntry{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err = d.authorize(ctx, req, storage.CredentialReference); err != nil {
		return domain.FileEntry{}, err
	}
	response, err := d.client.Do(req)
	if err != nil {
		return domain.FileEntry{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.FileEntry{}, fmt.Errorf("S3 stat returned %s", response.Status)
	}
	sz, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	return domain.FileEntry{StorageID: storage.ID, Path: key, Name: path.Base(key), Type: domain.FileEntryFile, SizeBytes: sz, ETag: strings.Trim(response.Header.Get("ETag"), "\""), Readable: true}, nil
}
func objectPrefix(value string) (string, error) {
	clean := strings.Trim(strings.TrimSpace(value), "/")
	if clean == "" {
		return "", nil
	}
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("S3 prefix cannot escape its root")
	}
	return clean + "/", nil
}

func (d *Driver) Put(ctx context.Context, request ports.PutObjectRequest) (domain.DataLocation, error) {
	objectURL, err := objectURL(request.Storage.Endpoint, request.Key)
	if err != nil {
		return domain.DataLocation{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, request.Source)
	if err != nil {
		return domain.DataLocation{}, err
	}
	if request.Size > 0 {
		httpRequest.ContentLength = request.Size
	}
	if err := d.authorize(ctx, httpRequest, request.Storage.CredentialReference); err != nil {
		return domain.DataLocation{}, err
	}
	if err := d.do(httpRequest, nil); err != nil {
		return domain.DataLocation{}, err
	}
	return domain.DataLocation{URI: objectURL, Status: domain.DataLocationAvailable,
		StorageResourceID: request.Storage.ID,
		Metadata:          map[string]any{"credentialReference": request.Storage.CredentialReference}}, nil
}

func (d *Driver) Get(ctx context.Context, request ports.GetObjectRequest) error {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, request.Location.URI, nil)
	if err != nil {
		return err
	}
	if err := d.authorize(ctx, httpRequest, metadataString(request.Location.Metadata, "credentialReference")); err != nil {
		return err
	}
	return d.do(httpRequest, request.Target)
}

func (d *Driver) Stat(ctx context.Context, location domain.DataLocation) (ports.ObjectStat, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, location.URI, nil)
	if err != nil {
		return ports.ObjectStat{}, err
	}
	if err := d.authorize(ctx, request, metadataString(location.Metadata, "credentialReference")); err != nil {
		return ports.ObjectStat{}, err
	}
	response, err := d.client.Do(request)
	if err != nil {
		return ports.ObjectStat{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ports.ObjectStat{}, fmt.Errorf("S3 stat returned %s", response.Status)
	}
	size, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	checksum := strings.Trim(response.Header.Get("ETag"), `"`)
	return ports.ObjectStat{SizeBytes: size, Checksum: checksum}, nil
}

func (d *Driver) Delete(ctx context.Context, location domain.DataLocation) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, location.URI, nil)
	if err != nil {
		return err
	}
	if err := d.authorize(ctx, request, metadataString(location.Metadata, "credentialReference")); err != nil {
		return err
	}
	return d.do(request, nil)
}

func (d *Driver) authorize(ctx context.Context, request *http.Request, reference string) error {
	if reference == "" || d.credentials == nil {
		return nil
	}
	if err := d.credentials.Authorize(ctx, request, reference); err != nil {
		return fmt.Errorf("resolve storage credential: %w", err)
	}
	return nil
}

func (d *Driver) do(request *http.Request, target io.Writer) error {
	response, err := d.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return fmt.Errorf("S3 request returned %s", response.Status)
	}
	if target != nil {
		_, err = io.Copy(target, response.Body)
	}
	return err
}

func objectURL(endpoint, key string) (string, error) {
	if strings.TrimSpace(endpoint) == "" {
		return "", fmt.Errorf("S3 endpoint is required")
	}
	segments, err := escapedSegments(key)
	if err != nil {
		return "", err
	}
	escaped := strings.Join(segments, "/")
	value := strings.ReplaceAll(endpoint, "{key}", escaped)
	if !strings.Contains(endpoint, "{key}") {
		value = strings.TrimRight(endpoint, "/") + "/" + escaped
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("S3 endpoint must be HTTP(S)")
	}
	return parsed.String(), nil
}

func escapedSegments(key string) ([]string, error) {
	parts := strings.Split(strings.Trim(key, "/"), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == ".." {
			return nil, fmt.Errorf("S3 object key cannot escape its root")
		}
		if part != "" && part != "." {
			result = append(result, url.PathEscape(part))
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("S3 object key is required")
	}
	return result, nil
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return value
}
