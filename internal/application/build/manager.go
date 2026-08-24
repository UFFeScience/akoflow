package build

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
)

// Manager owns immutable context bytes below Root. No caller supplied path or
// URI is ever used to locate a build context.
type Manager struct {
	Root     string
	MaxBytes int64
	Catalog  Catalog
	Executor Executor
}

func (m Manager) MaxUploadBytes() int64 {
	if m.MaxBytes > 0 {
		return m.MaxBytes
	}
	return 512 << 20
}

func (m Manager) OpenOutput(_ context.Context, runID string) (io.ReadCloser, string, error) {
	if m.Root == "" || runID == "" || filepath.Base(runID) != runID {
		return nil, "", fmt.Errorf("invalid build output")
	}
	name := runID + ".sif"
	file, err := os.Open(filepath.Join(m.Root, "outputs", name))
	if err != nil {
		return nil, "", fmt.Errorf("SIF output is unavailable: %w", err)
	}
	return file, name, nil
}

func (m Manager) Upload(ctx context.Context, input io.Reader) (domain.BuildContextArtifact, error) {
	if m.Root == "" {
		return domain.BuildContextArtifact{}, fmt.Errorf("artifact store is not configured")
	}
	if m.MaxBytes <= 0 {
		m.MaxBytes = 512 << 20
	}
	if err := os.MkdirAll(filepath.Join(m.Root, "contexts", ".tmp"), 0700); err != nil {
		return domain.BuildContextArtifact{}, err
	}
	tmp, err := os.CreateTemp(filepath.Join(m.Root, "contexts", ".tmp"), "upload-*.tar.gz")
	if err != nil {
		return domain.BuildContextArtifact{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(input, m.MaxBytes+1))
	closeErr := tmp.Close()
	if err != nil {
		return domain.BuildContextArtifact{}, err
	}
	if closeErr != nil {
		return domain.BuildContextArtifact{}, closeErr
	}
	if n == 0 || n > m.MaxBytes {
		return domain.BuildContextArtifact{}, fmt.Errorf("build context must be between 1 and %d bytes", m.MaxBytes)
	}
	if err := validGzipTar(tmpName); err != nil {
		return domain.BuildContextArtifact{}, err
	}
	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	final := filepath.Join(m.Root, "contexts", strings.TrimPrefix(digest, "sha256:")+".tar.gz")
	if _, err := os.Stat(final); os.IsNotExist(err) {
		if err := os.Rename(tmpName, final); err != nil {
			return domain.BuildContextArtifact{}, err
		}
	}
	v := domain.BuildContextArtifact{Digest: digest, StorageURI: "artifact://contexts/" + strings.TrimPrefix(digest, "sha256:"), SizeBytes: n, MediaType: "application/vnd.akoflow.build-context+tar+gzip"}
	if m.Catalog != nil {
		old, err := m.Catalog.FindBuildContext(ctx, digest)
		if err != nil {
			return domain.BuildContextArtifact{}, err
		}
		if old == nil {
			if err := m.Catalog.SaveBuildContext(ctx, v); err != nil {
				return domain.BuildContextArtifact{}, err
			}
		}
	}
	return v, nil
}
func validGzipTar(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("build context must be a gzip stream: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return fmt.Errorf("read build context: %w", e)
		}
		if !safeArchivePath(h.Name) || h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink {
			return fmt.Errorf("unsafe archive path %q", h.Name)
		}
	}
}
func safeArchivePath(name string) bool {
	clean := filepath.Clean(name)
	return name != "" && !filepath.IsAbs(name) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
func (m Manager) ResolveBuildContext(_ context.Context, digest string) (string, error) {
	if !strings.HasPrefix(digest, "sha256:") || len(digest) != 71 {
		return "", fmt.Errorf("invalid build context digest")
	}
	archive := filepath.Join(m.Root, "contexts", strings.TrimPrefix(digest, "sha256:")+".tar.gz")
	if _, err := os.Stat(archive); err != nil {
		return "", fmt.Errorf("build context unavailable: %w", err)
	}
	dir := filepath.Join(m.Root, "workspaces", strings.TrimPrefix(digest, "sha256:"))
	if _, err := os.Stat(dir); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			_ = os.RemoveAll(dir)
			return "", e
		}
		if !safeArchivePath(h.Name) || h.Typeflag == tar.TypeSymlink || h.Typeflag == tar.TypeLink {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("unsafe archive path %q", h.Name)
		}
		target := filepath.Join(dir, h.Name)
		if h.FileInfo().IsDir() {
			if e = os.MkdirAll(target, 0700); e != nil {
				return "", e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return "", e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if e != nil {
			return "", e
		}
		_, e = io.Copy(out, tr)
		ce := out.Close()
		if e != nil || ce != nil {
			return "", fmt.Errorf("extract build context: %v", e)
		}
	}
	return dir, nil
}
func (m Manager) Start(ctx context.Context, spec domain.ArtifactBuild) (domain.BuildRun, error) {
	run := domain.BuildRun{ID: provider.NewID("build-run"), ArtifactBuildID: spec.ID, Status: "queued"}
	if err := m.Catalog.SaveBuildRun(ctx, run); err != nil {
		return run, err
	}
	go func() { _, _ = m.Executor.Execute(context.Background(), spec, run) }()
	return run, nil
}
