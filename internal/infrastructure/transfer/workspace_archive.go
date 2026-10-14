package transfer

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func measureWorkspacePhase(run *domain.DataTransferRun, name string, operation func() error) error {
	start := time.Now()
	err := operation()
	end := time.Now()
	run.Route.Phases = append(run.Route.Phases, domain.TransferPhase{
		Name: name, StartedAt: float64(start.UnixNano()) / 1e9,
		FinishedAt:      float64(end.UnixNano()) / 1e9,
		DurationSeconds: end.Sub(start).Seconds(), Failed: err != nil,
	})
	return err
}

// Record admission separately from destination command service. A zero-valued
// pointer proves the queue was measured; absence keeps legacy wall-time data
// distinguishable, so calibration cannot silently double-count contention.
func measureWorkspaceSSHPhase(ctx context.Context, run *domain.DataTransferRun, name string, endpoint domain.TransferEndpoint, args []string) ([]byte, error) {
	var output []byte
	var queue, execution float64
	var executed bool
	err := measureWorkspacePhase(run, name, func() error {
		queuedAt := time.Now()
		command, release, err := queuedSSHCommand(ctx, endpoint, args)
		queue = time.Since(queuedAt).Seconds()
		if err != nil {
			return err
		}
		defer release()
		startedAt := time.Now()
		output, err = command.CombinedOutput()
		execution, executed = time.Since(startedAt).Seconds(), true
		return err
	})
	phase := &run.Route.Phases[len(run.Route.Phases)-1]
	phase.SSHChannelWaitSeconds = &queue
	if executed {
		phase.ExecutionSeconds = &execution
	}
	return output, err
}

func parallelWorkspacePreparation(ctx context.Context, count, limit int, prepare func(context.Context, int) error) error {
	if limit <= 0 {
		limit = 8
	}
	if limit > 8 {
		limit = 8
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	slots := make(chan struct{}, limit)
	var workers sync.WaitGroup
	var once sync.Once
	var firstError error
	for i := 0; i < count; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			if ctx.Err() != nil {
				return
			}
			if err := prepare(ctx, i); err != nil {
				once.Do(func() { firstError = err; cancel() })
			}
		}(i)
	}
	workers.Wait()
	if firstError != nil {
		return firstError
	}
	return ctx.Err()
}

// Archives contain snapshots, never a directly published consumer workspace.
// Validate every member locally and verify the archive digest on the receiver
// before extraction. The existing aggregate content/conflict barrier still runs.
func relayWorkspaceArchive(ctx context.Context, source, destination domain.TransferEndpoint, run *domain.DataTransferRun) error {
	root, err := os.MkdirTemp("", "akoflow-archive-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	archivePath := filepath.Join(root, "workspace.tar.gz")
	if err := receiveRelayWorkspaceArchive(ctx, source, archivePath, run); err != nil {
		return err
	}
	var digest string
	err = measureWorkspacePhase(run, "archive-validation", func() error {
		var validationErr error
		digest, validationErr = validateWorkspaceArchive(archivePath)
		return validationErr
	})
	if err != nil {
		return err
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return err
	}
	if strings.HasPrefix(source.URI, "ssh://") {
		run.NetworkBytes += info.Size()
	}
	run.Route.ArchiveBytes = info.Size()
	remoteURL, err := url.Parse(destination.URI)
	if err != nil {
		return err
	}
	remoteURL.Path += ".archive"
	spool := destination
	spool.URI = remoteURL.String()
	if err := makeWorkspaceDirectory(ctx, spool); err != nil {
		return err
	}
	local := domain.TransferEndpoint{URI: (&url.URL{Scheme: "file", Path: root}).String()}
	var stats rsyncStats
	err = measureWorkspacePhase(run, "archive-upload", func() error {
		var uploadErr error
		stats, uploadErr = syncDirectory(ctx, local, spool, false)
		return uploadErr
	})
	if stats.executionSeconds > 0 {
		phase := &run.Route.Phases[len(run.Route.Phases)-1]
		phase.SSHChannelWaitSeconds = &stats.channelWaitSeconds
		phase.ExecutionSeconds = &stats.executionSeconds
	}
	recordWorkspaceRsyncStats(run, stats, true)
	if err != nil {
		return err
	}
	if err := makeWorkspaceDirectory(ctx, destination); err != nil {
		return err
	}
	host, target, err := sshTarget(destination, "")
	if err != nil {
		return err
	}
	_, spoolPath, err := sshTarget(spool, "")
	if err != nil {
		return err
	}
	remoteArchive := filepath.Join(spoolPath, "workspace.tar.gz")
	command := "set -eu; actual=$(sha256sum -- " + shell(remoteArchive) + "); test \"${actual%% *}\" = " + shell(digest) +
		"; tar --extract --file=" + shell(remoteArchive) + " --directory=" + shell(target) + " --no-same-owner --no-same-permissions"
	var output []byte
	output, err = measureWorkspaceSSHPhase(ctx, run, "receiver-verify-and-extract", destination, append(sshArgs(destination), host, command))
	if err != nil {
		return fmt.Errorf("verify/extract workspace archive: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return rejectWorkspaceSymlinks(ctx, destination)
}

func packWorkspaceArchive(ctx context.Context, source domain.TransferEndpoint, archivePath string) error {
	return packWorkspaceArchiveWithTiming(ctx, source, archivePath, &workspaceArchiveTiming{})
}

type workspaceArchiveTiming struct {
	PreflightSeconds            float64
	PreflightChannelWaitSeconds float64
	ChannelWaitSeconds          float64
	ExecutionSeconds            float64
}

func packWorkspaceArchiveWithTiming(ctx context.Context, source domain.TransferEndpoint, archivePath string, timing *workspaceArchiveTiming) error {
	preflightStarted := time.Now()
	preflightErr := rejectWorkspaceSymlinksWithWait(ctx, source, &timing.PreflightChannelWaitSeconds)
	timing.PreflightSeconds = time.Since(preflightStarted).Seconds()
	if preflightErr != nil {
		return preflightErr
	}
	u, err := url.Parse(source.URI)
	if err != nil {
		return err
	}
	var command *exec.Cmd
	release := func() {}
	switch u.Scheme {
	case "file":
		started := time.Now()
		defer func() { timing.ExecutionSeconds = time.Since(started).Seconds() }()
		return packLocalWorkspaceArchive(ctx, u.Path, archivePath)
	case "ssh":
		host, origin, err := sshTarget(source, "")
		if err != nil {
			return err
		}
		queuedAt := time.Now()
		command, release, err = queuedSSHCommand(ctx, source, append(sshArgs(source), host,
			"set -eu; bad=$(find "+shell(origin)+" ! -type d ! -type f -print -quit); test -z \"$bad\"; bash -o pipefail -c "+shell("tar --hard-dereference -C \"$1\" -cf - . | gzip -1")+" archive "+shell(origin)))
		timing.ChannelWaitSeconds = time.Since(queuedAt).Seconds()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported archive source scheme %q", u.Scheme)
	}
	defer release()
	file, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	var stderr strings.Builder
	command.Stdout, command.Stderr = file, &stderr
	executionStarted := time.Now()
	defer func() { timing.ExecutionSeconds = time.Since(executionStarted).Seconds() }()
	if err := command.Run(); err != nil {
		return fmt.Errorf("pack workspace archive: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return file.Close()
}

func packLocalWorkspaceArchive(ctx context.Context, root, filename string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewWriterLevel(file, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer compressed.Close()
	writer := tar.NewWriter(compressed)
	defer writer.Close()
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported workspace entry %q", name)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		input, err := os.Open(name)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := compressed.Close(); err != nil {
		return err
	}
	return file.Close()
}

func validateWorkspaceArchive(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	transport := bufio.NewReader(io.TeeReader(file, hash))
	var reader io.Reader = transport
	magic, _ := transport.Peek(2)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		compressed, err := gzip.NewReader(transport)
		if err != nil {
			return "", fmt.Errorf("invalid compressed workspace archive: %w", err)
		}
		defer compressed.Close()
		// BSD tar pads a gzip transport with zero blocks. Do not interpret this
		// padding as a second gzip stream or accept unvalidated extra members.
		compressed.Multistream(false)
		reader = compressed
	}
	archive := tar.NewReader(reader)
	seen := make(map[string]byte)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("invalid workspace archive: %w", err)
		}
		name := path.Clean(header.Name)
		if path.IsAbs(header.Name) || name == ".." || strings.HasPrefix(name, "../") || strings.Contains("/"+header.Name+"/", "/../") {
			return "", fmt.Errorf("unsafe archive path %q", header.Name)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return "", fmt.Errorf("unsupported archive entry %q (links and special files forbidden)", header.Name)
		}
		if name == "." && header.Typeflag != tar.TypeDir {
			return "", fmt.Errorf("invalid archive root")
		}
		if _, exists := seen[name]; exists {
			return "", fmt.Errorf("duplicate archive path %q", name)
		}
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if kind, exists := seen[parent]; exists && kind != tar.TypeDir {
				return "", fmt.Errorf("archive parent is a file: %q", parent)
			}
		}
		seen[name] = header.Typeflag
	}
	// Drain the decompressor to validate its checksum/trailer, then include any
	// remaining transport bytes in the digest verified before remote extraction.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return "", fmt.Errorf("invalid archive trailer: %w", err)
	}
	for {
		b, err := transport.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if b != 0 {
			return "", fmt.Errorf("unexpected data after workspace archive")
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func receiveRelayWorkspaceArchive(ctx context.Context, source domain.TransferEndpoint, archivePath string, run *domain.DataTransferRun) error {
	if err := measureWorkspacePhase(run, "pack-and-receive", func() error {
		var timing workspaceArchiveTiming
		defer func() {
			run.Route.SourcePackPreflightSeconds = timing.PreflightSeconds
			run.Route.SourcePackPreflightSSHChannelWaitSeconds = timing.PreflightChannelWaitSeconds
			run.Route.SourcePackPreflightServiceSeconds = max(0, timing.PreflightSeconds-timing.PreflightChannelWaitSeconds)
			run.Route.SourcePackSSHChannelWaitSeconds = timing.ChannelWaitSeconds
			run.Route.SourcePackExecutionSeconds = timing.ExecutionSeconds
		}()
		return packWorkspaceArchiveWithTiming(ctx, source, archivePath, &timing)
	}); err != nil {
		if strings.HasPrefix(source.URI, "ssh://") {
			if partial, statErr := os.Stat(archivePath); statErr == nil {
				run.NetworkBytes += partial.Size()
			}
			run.Route.NetworkBytesIncomplete = true
		}
		return err
	}
	return nil
}
