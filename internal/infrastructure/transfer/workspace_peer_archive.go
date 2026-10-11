package transfer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

var peerArchiveDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

// preparePeerWorkspaceArchive keeps payload on the two workers. The probe
// writes only an empty file in the private preparation directory. Failure of
// that probe leaves the gateway fallback available; payload/validation errors
// after a successful probe are surfaced, never silently retried as another route.
func preparePeerWorkspaceArchive(ctx context.Context, source, destination domain.TransferEndpoint, run *domain.DataTransferRun) (bool, error) {
	if !cloudWorkspacePeerEligible(source, destination) {
		return false, nil
	}
	sourceHost, _, err := sshTarget(source, "")
	if err != nil {
		return true, err
	}
	targetHost, target, err := sshTarget(destination, "")
	if err != nil {
		return true, err
	}
	output, err := runSSHStdout(ctx, source, append(sshArgs(source), sourceHost, "mktemp -d /tmp/akoflow-peer-XXXXXX"), nil)
	if err != nil {
		return false, nil
	}
	root := strings.TrimSpace(string(output))
	if !filepath.IsAbs(root) || filepath.Dir(root) != "/tmp" || !strings.HasPrefix(filepath.Base(root), "akoflow-peer-") {
		return true, fmt.Errorf("invalid peer preparation directory")
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_, _ = runSSHCombinedOutput(cleanupCtx, source, append(sshArgs(source), sourceHost, "rm -rf -- "+shell(root)), nil)
	}()
	archiveSource := source
	u, _ := url.Parse(source.URI)
	u.Path = root
	archiveSource.URI = u.String()
	if err := makeWorkspaceDirectory(ctx, destination); err != nil {
		return true, err
	}
	if _, err := runSSHCombinedOutput(ctx, source, append(sshArgs(source), sourceHost, "umask 077; touch -- "+shell(filepath.Join(root, "probe"))), nil); err != nil {
		return true, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	probeErr := measureWorkspacePhase(run, "peer-authenticated-connectivity-probe", func() error {
		_, err := (RsyncSSH{}).TransferRoute(probeCtx, domain.TransferDirectRuntime, archiveSource, destination, "probe", "peer-probe", 0)
		return err
	})
	cancel()
	if probeErr != nil {
		run.Route.PeerProbeFailure = peerProbeFailureCategory(probeErr)
		cleanupCtx, release := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		_, cleanupErr := runSSHCombinedOutput(cleanupCtx, destination, append(sshArgs(destination), targetHost, "rm -f -- "+shell(filepath.Join(target, "peer-probe"))), nil)
		release()
		if cleanupErr != nil {
			return true, fmt.Errorf("clean failed peer probe: %w", cleanupErr)
		}
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		run.Route.Fallback = domain.TransferGateway
		run.Route.Reason = "private peer authentication/connectivity not proven; gateway fallback"
		return false, nil
	}
	if err := rejectWorkspaceSymlinks(ctx, source); err != nil {
		return true, err
	}
	var snapshot *peerSnapshot
	var release func()
	err = measureWorkspacePhase(run, "peer-snapshot-acquire", func() error {
		var reused bool
		var acquireErr error
		acquireStarted := time.Now()
		snapshot, reused, release, acquireErr = sharedPeerSnapshots.acquire(ctx, peerSnapshotKey(source), func(buildCtx context.Context) (*peerSnapshot, error) {
			return buildPeerSnapshot(buildCtx, source)
		})
		run.Route.SnapshotReused = reused
		if acquireErr == nil && snapshot != nil {
			run.Route.SnapshotBuildWallSeconds = snapshot.buildWallSeconds
			waitSeconds, commandSeconds := snapshot.buildSSHWaitSeconds, snapshot.buildCommandSeconds
			run.Route.SnapshotBuildSSHWaitSeconds = &waitSeconds
			run.Route.SnapshotBuildCommandSeconds = &commandSeconds
			if snapshot.serviceTiming != nil {
				serviceSeconds, cpuSeconds := snapshot.serviceTiming.WallSeconds, snapshot.serviceTiming.CPUSeconds
				run.Route.SnapshotBuildServiceSeconds = &serviceSeconds
				run.Route.SnapshotBuildCPUSeconds = &cpuSeconds
			}
			wait := time.Since(acquireStarted).Seconds()
			if !reused {
				wait -= snapshot.buildWallSeconds
			}
			if wait < 0 {
				wait = 0
			}
			run.Route.SnapshotAcquireWaitSeconds = wait
		}
		return acquireErr
	})
	if err != nil {
		return true, err
	}
	defer release()
	if run.Route.SnapshotReused {
		if err := measureWorkspacePhase(run, "peer-reused-source-manifest-check", func() error { return verifyPeerSnapshotSource(ctx, source) }); err != nil {
			return true, err
		}
	}
	u.Path = snapshot.root
	archiveSource.URI = u.String()
	run.Route.ArchiveBytes = snapshot.size
	err = measureWorkspacePhase(run, "peer-private-upload", func() error {
		_, err := (RsyncSSH{}).TransferRoute(ctx, domain.TransferDirectRuntime, archiveSource, destination, "workspace.tar.gz", "peer-workspace.tar.gz", 0)
		return err
	})
	if err != nil {
		run.Route.NetworkBytesIncomplete = true
		return true, err
	}
	run.NetworkBytes = snapshot.size
	command := "python3 -c " + shell(peerArchiveVerifyScript) + " " + shell(filepath.Join(target, "peer-workspace.tar.gz")) + " " + shell(target) + " " + shell(snapshot.digest)
	command = "python3 -c " + shell(peerSnapshotTimingScript) + " " + shell(command)
	var receiverWait, receiverCommand float64
	err = measureWorkspacePhase(run, "peer-receiver-verify-and-extract", func() error {
		var commandErr error
		output, commandErr = runSSHProtocolMeasured(ctx, destination, append(sshArgs(destination), targetHost, command), &receiverWait, &receiverCommand)
		return commandErr
	})
	phase := &run.Route.Phases[len(run.Route.Phases)-1]
	phase.SSHChannelWaitSeconds = &receiverWait
	phase.ExecutionSeconds = &receiverCommand
	if err != nil {
		return true, fmt.Errorf("verify private peer archive: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		timing, timingErr := parsePeerSnapshotServiceTiming(strings.TrimSpace(string(output)))
		if timingErr != nil {
			return true, timingErr
		}
		run.Route.PeerReceiverServiceSeconds = &timing.WallSeconds
		run.Route.PeerReceiverCPUSeconds = &timing.CPUSeconds
	}
	return true, rejectWorkspaceSymlinks(ctx, destination)
}

// Persist only bounded categories: SSH stderr may contain addresses, paths or
// remote diagnostic text that must not be promoted into scientific metadata.
func peerProbeFailureCategory(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "probe_timeout"
	}
	message := strings.ToLower(err.Error())
	for _, stage := range []struct{ fragment, category string }{
		{"install temporary destination credential", "destination_credential_installation_failed"},
		{"install temporary source credential", "source_credential_installation_failed"},
		{"install verified direct destination host key", "source_host_key_installation_failed"},
		{"read verified direct destination host key", "verified_host_key_unavailable"},
		{"host key verification failed", "host_key_rejected"},
		{"invalid format", "identity_format_rejected"},
		{"error in libcrypto", "identity_format_rejected"},
		{"permission denied", "authentication_rejected"},
		{"connection refused", "connection_refused"},
		{"timed out", "probe_timeout"},
	} {
		if strings.Contains(message, stage.fragment) {
			return stage.category
		}
	}
	return "ssh_probe_failed"
}

// Validate every member before publishing even a single extracted file. Only
// regular files and directories are accepted; links, duplicates and path
// traversal cannot escape the private, mktemp-based merge preparation root.
const peerArchiveVerifyScript = `import gzip, hashlib, os, posixpath, shutil, sys, tarfile
archive, root, digest = sys.argv[1:]
hasher = hashlib.sha256()
with open(archive, 'rb') as f:
    for block in iter(lambda: f.read(65536), b''): hasher.update(block)
if hasher.hexdigest() != digest: raise ValueError('peer archive digest mismatch')
with gzip.open(archive, 'rb') as stream:
    with tarfile.open(fileobj=stream, mode='r:') as tar:
        members = tar.getmembers()
        seen = {}
        for member in members:
            name = posixpath.normpath(member.name)
            if member.name.startswith('/') or '..' in member.name.split('/') or name == '..' or name.startswith('../'):
                raise ValueError('unsafe peer archive path')
            if member.type not in (tarfile.REGTYPE, tarfile.AREGTYPE, tarfile.DIRTYPE) or name in seen or (name == '.' and not member.isdir()):
                raise ValueError('unsupported or duplicate peer archive member')
            seen[name] = member.isdir()
        for name in seen:
            parent = posixpath.dirname(name)
            while parent not in ('', '.'):
                if parent in seen and not seen[parent]: raise ValueError('archive parent is a file')
                parent = posixpath.dirname(parent)
        for block in iter(lambda: stream.read(65536), b''):
            if any(block): raise ValueError('unexpected peer archive trailer')
        # Use a separate extraction tree so the probe/archive themselves never
        # become scientific workspace inputs.
        output = os.path.join(root, 'contents')
        os.mkdir(output, 0o700)
        # Reopen sequentially after complete validation. Random extractfile
        # access on gzip otherwise replays decompression after the scan.
with tarfile.open(archive, mode='r|gz') as tar:
        for member in tar:
            name = posixpath.normpath(member.name)
            if name == '.': continue
            destination = os.path.join(output, name)
            if member.isdir():
                os.makedirs(destination, mode=0o700, exist_ok=True)
            else:
                os.makedirs(os.path.dirname(destination), mode=0o700, exist_ok=True)
                fd = os.open(destination, os.O_CREAT | os.O_EXCL | os.O_WRONLY | os.O_NOFOLLOW, member.mode & 0o777)
                with os.fdopen(fd, 'wb') as f, tar.extractfile(member) as source: shutil.copyfileobj(source, f, 65536)
os.unlink(archive)
os.unlink(os.path.join(root, 'peer-probe'))
`
