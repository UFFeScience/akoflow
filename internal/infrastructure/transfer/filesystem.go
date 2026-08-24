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

// LocalFilesystem is also useful for mounted NFS/PVC and gateway staging.
type LocalFilesystem struct{}

func (LocalFilesystem) CanHandle(e domain.TransferEndpoint) bool {
	u, err := url.Parse(e.URI)
	return err == nil && (u.Scheme == "" || u.Scheme == "file")
}
func localPath(e domain.TransferEndpoint, name string) (string, error) {
	u, err := url.Parse(e.URI)
	if err != nil {
		return "", err
	}
	if u.Scheme != "" && u.Scheme != "file" {
		return "", fmt.Errorf("not a file endpoint")
	}
	base := u.Path
	if base == "" {
		base = e.URI
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return "", err
	}
	base = filepath.Clean(base)
	if name == "" {
		return base, nil
	}
	// Endpoint names are logical relative keys; accepting an absolute path or
	// traversal here would allow a transfer plan to write outside its endpoint.
	key := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(key) || key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("transfer path escapes endpoint")
	}
	full := filepath.Join(base, key)
	parent := filepath.Dir(full)
	// Resolve an existing parent so a symlink in a staging tree cannot redirect
	// the transfer outside the configured endpoint.
	for {
		_, statErr := os.Lstat(parent)
		if statErr == nil {
			break
		}
		if !os.IsNotExist(statErr) {
			return "", statErr
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", fmt.Errorf("endpoint has no existing parent")
		}
		parent = next
	}
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	if realParent != realBase && !strings.HasPrefix(realParent, realBase+string(filepath.Separator)) {
		return "", fmt.Errorf("transfer path escapes endpoint via symlink")
	}
	return full, nil
}
func (LocalFilesystem) Exists(_ context.Context, e domain.TransferEndpoint, name string) (bool, error) {
	p, err := localPath(e, name)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	return err == nil, nil
}
func (LocalFilesystem) Open(_ context.Context, e domain.TransferEndpoint, name string, offset int64) (io.ReadCloser, error) {
	p, err := localPath(e, name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err == nil && offset > 0 {
		_, err = f.Seek(offset, 0)
	}
	return f, err
}
func (LocalFilesystem) Put(_ context.Context, e domain.TransferEndpoint, name string, r io.Reader, offset int64) error {
	p, err := localPath(e, name)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if offset == 0 {
		if err = f.Truncate(0); err == nil {
			_, err = f.Seek(0, 0)
		}
	} else {
		_, err = f.Seek(offset, 0)
	}
	if err != nil {
		_ = f.Close()
		return err
	}
	_, err = io.Copy(f, r)
	cerr := f.Close()
	if err != nil {
		return err
	}
	return cerr
}
func (LocalFilesystem) Commit(_ context.Context, e domain.TransferEndpoint, partial, final string) error {
	p, err := localPath(e, partial)
	if err != nil {
		return err
	}
	f, err := localPath(e, final)
	if err != nil {
		return err
	}
	return os.Rename(p, f)
}
