package transfer

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// ArtifactStore exposes completed, control-plane-managed build outputs as a
// read-only transfer source. artifact:// is intentionally not a generic URL:
// every path is confined to the configured artifact store root.
type ArtifactStore struct{ Root string }

func (s ArtifactStore) CanHandle(endpoint domain.TransferEndpoint) bool {
	uri, err := url.Parse(endpoint.URI)
	return err == nil && uri.Scheme == "artifact"
}

func (s ArtifactStore) file(endpoint domain.TransferEndpoint) (string, error) {
	if s.Root == "" {
		return "", fmt.Errorf("artifact store root is not configured")
	}
	uri, err := url.Parse(endpoint.URI)
	if err != nil || uri.Scheme != "artifact" {
		return "", fmt.Errorf("not an artifact endpoint")
	}
	key := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(uri.Host+uri.Path, "/")))
	if key == "." || filepath.IsAbs(key) || key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path escapes store root")
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, key), nil
}

func (s ArtifactStore) Exists(_ context.Context, endpoint domain.TransferEndpoint, _ string) (bool, error) {
	path, err := s.file(endpoint)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	return err == nil, nil
}

func (s ArtifactStore) Open(_ context.Context, endpoint domain.TransferEndpoint, _ string, offset int64) (io.ReadCloser, error) {
	path, err := s.file(endpoint)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err = file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}

func (ArtifactStore) Put(context.Context, domain.TransferEndpoint, string, io.Reader, int64) error {
	return fmt.Errorf("artifact store is read-only")
}

func (ArtifactStore) Commit(context.Context, domain.TransferEndpoint, string, string) error {
	return fmt.Errorf("artifact store is read-only")
}
