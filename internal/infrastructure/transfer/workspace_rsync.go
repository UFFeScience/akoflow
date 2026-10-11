package transfer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
)

// WorkspaceRsync stages complete predecessor snapshots before publishing them.
// In particular, no predecessor is allowed to overwrite a different file from
// another predecessor or an already populated successor workspace.
type WorkspaceRsync struct {
	// Parallelism bounds independent preparations; merge remains a single barrier.
	Parallelism int
	Resolver    interface {
		ResolveTransferEndpoint(context.Context, domain.TransferLocation) (domain.TransferEndpoint, error)
	}
}

func (s WorkspaceRsync) Sync(ctx context.Context, plans []domain.DataTransferPlan) ([]domain.DataTransferRun, error) {
	runs := make([]domain.DataTransferRun, len(plans))
	start := float64(time.Now().UnixNano()) / 1e9
	for i, plan := range plans {
		runs[i] = domain.DataTransferRun{ID: plan.ID, PlanID: plan.ID, ExecutionRunID: plan.ExecutionRunID,
			ActivityID: plan.ConsumerActivityID, Status: domain.TransferRunning, Strategy: domain.TransferGateway,
			StartedAt: start, Route: domain.TransferRoute{Strategy: domain.TransferGateway,
				ProducerActivityID: plan.ProducerActivityID, ConsumerActivityID: plan.ConsumerActivityID,
				SourceResourceID: plan.Source.ResourceID, TargetResourceID: plan.Destination.ResourceID,
				SourceEnvironmentID: plan.Source.EnvironmentID, TargetEnvironmentID: plan.Destination.EnvironmentID,
				SourceCloudInstanceID: plan.Source.CloudInstanceID, TargetCloudInstanceID: plan.Destination.CloudInstanceID,
				Reason: "incremental rsync of predecessor workspace through verified staging"}}
	}
	fail := func(err error) ([]domain.DataTransferRun, error) {
		end := float64(time.Now().UnixNano()) / 1e9
		for i := range runs {
			if runs[i].Status == domain.TransferCompleted {
				continue
			}
			runs[i].Status, runs[i].FinishedAt, runs[i].Error = domain.TransferFailed, end, err.Error()
			runs[i].DurationSeconds = max(0, end-runs[i].StartedAt)
		}
		return runs, err
	}
	if len(plans) == 0 {
		return runs, nil
	}
	if s.Resolver == nil {
		return fail(fmt.Errorf("workspace endpoint resolver is unavailable"))
	}
	sources, destination, direct, err := s.directRemoteEndpoints(ctx, plans)
	if err != nil {
		return fail(err)
	}
	if direct {
		result, syncErr := s.syncDependencyRoutes(ctx, plans, sources, destination, runs)
		if syncErr != nil {
			return fail(syncErr)
		}
		return result, nil
	}
	root, err := os.MkdirTemp("", "akoflow-workspace-sync-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(root)
	stages, ingress, err := s.stageProducers(ctx, plans, root, runs)
	if err != nil {
		return fail(err)
	}
	if err := makeWorkspaceDirectory(ctx, destination); err != nil {
		return fail(err)
	}
	if err := rejectWorkspaceSymlinks(ctx, destination); err != nil {
		return fail(err)
	}
	if err := publishWorkspace(ctx, plans, stages, ingress, destination, runs); err != nil {
		return fail(err)
	}
	return runs, nil
}

func (s WorkspaceRsync) directRemoteEndpoints(ctx context.Context, plans []domain.DataTransferPlan) ([]domain.TransferEndpoint, domain.TransferEndpoint, bool, error) {
	destination, err := s.Resolver.ResolveTransferEndpoint(ctx, plans[0].Destination)
	if err != nil {
		return nil, domain.TransferEndpoint{}, false, fmt.Errorf("resolve successor workspace: %w", err)
	}
	if !strings.HasPrefix(destination.URI, "ssh://") || destination.ConnectionID == "" {
		return nil, destination, false, nil
	}
	sources := make([]domain.TransferEndpoint, len(plans))
	for index, plan := range plans {
		source, resolveErr := s.Resolver.ResolveTransferEndpoint(ctx, plan.Source)
		if resolveErr != nil {
			return nil, destination, false, fmt.Errorf("resolve producer %q: %w", plan.ProducerActivityID, resolveErr)
		}
		sources[index] = source
	}
	// Remote merge can combine independently prepared network snapshots with
	// proven shared-filesystem sources; a mixed origin never downgrades all routes.
	return sources, destination, true, nil
}

func sameSSHWorkspaceEndpoint(source, destination domain.TransferEndpoint) bool {
	if source.ConnectionID == "" || source.ConnectionID != destination.ConnectionID {
		return false
	}
	sourceURL, sourceErr := url.Parse(source.URI)
	destinationURL, destinationErr := url.Parse(destination.URI)
	if sourceErr != nil || destinationErr != nil || sourceURL.Scheme != "ssh" || destinationURL.Scheme != "ssh" {
		return false
	}
	return sourceURL.Host == destinationURL.Host && sourceURL.User.String() == destinationURL.User.String()
}

func syncRemoteWorkspaceDirect(ctx context.Context, plans []domain.DataTransferPlan, sources []domain.TransferEndpoint, destination domain.TransferEndpoint, runs []domain.DataTransferRun) ([]domain.DataTransferRun, error) {
	host, destinationPath, err := sshTarget(destination, "")
	if err != nil {
		return runs, err
	}
	sourcePaths := make([]string, len(sources))
	for index, source := range sources {
		_, sourcePath, sourceErr := sshTarget(source, "")
		if sourceErr != nil {
			return runs, sourceErr
		}
		sourcePaths[index] = sourcePath
	}
	script := remoteWorkspaceScript(sourcePaths, destinationPath)
	mergeStart := time.Now()
	command, release, err := queuedSSHCommand(ctx, destination, append(sshArgs(destination), host, "bash -s"))
	admittedAt := time.Now()
	var output []byte
	if err == nil {
		command.Stdin = strings.NewReader(script)
		output, err = command.CombinedOutput()
		release()
	}
	mergeEnd := time.Now()
	for i := range runs {
		runs[i].Route.MergeSSHChannelWaitSeconds = admittedAt.Sub(mergeStart).Seconds()
		if command != nil {
			runs[i].Route.MergeExecutionSeconds = mergeEnd.Sub(admittedAt).Seconds()
		}
		runs[i].Route.MergeCheckSeconds = remoteMergeSeconds(string(output), "CHECK")
		runs[i].Route.MergeCopySeconds = remoteMergeSeconds(string(output), "COPY")
		runs[i].Route.MergeVerifySeconds = remoteMergeSeconds(string(output), "VERIFY")
		// Residual command wall time includes transport/remote startup and
		// untimed script work; it is not network throughput or service queue.
		runs[i].Route.MergeCommandOverheadSeconds = max(0, runs[i].Route.MergeExecutionSeconds-
			runs[i].Route.MergeCheckSeconds-runs[i].Route.MergeCopySeconds-runs[i].Route.MergeVerifySeconds)
		runs[i].Route.Phases = append(runs[i].Route.Phases, domain.TransferPhase{
			Name: "shared-conflict-check-copy-and-verify", StartedAt: float64(mergeStart.UnixNano()) / 1e9,
			FinishedAt: float64(mergeEnd.UnixNano()) / 1e9, DurationSeconds: mergeEnd.Sub(mergeStart).Seconds(), Failed: err != nil,
		})
	}
	if err != nil {
		return runs, fmt.Errorf("direct remote workspace sync: %w: %s", err, strings.TrimSpace(string(output)))
	}
	now := float64(time.Now().UnixNano()) / 1e9
	for index := range runs {
		stats := parseRsyncStats(remoteWorkspaceStats(string(output), index), false)
		runs[index].Strategy = domain.TransferDirectRuntime
		runs[index].Route.Strategy = domain.TransferDirectRuntime
		runs[index].Route.Reason = "direct workspace copy on shared HPC filesystem"
		runs[index].LogicalBytes = stats.fileBytes
		runs[index].TransferredBytes = stats.fileBytes
		runs[index].FilesTransferred = stats.files
		runs[index].NetworkBytes = 0
		runs[index].FinishedAt = now
		runs[index].DurationSeconds = max(0, now-runs[index].StartedAt)
		runs[index].Status = domain.TransferCompleted
	}
	return runs, nil
}

func remoteWorkspaceStats(output string, index int) string {
	begin := fmt.Sprintf("__AKOFLOW_WORKSPACE_BEGIN_%d__", index)
	end := fmt.Sprintf("__AKOFLOW_WORKSPACE_END_%d__", index)
	start := strings.Index(output, begin)
	if start < 0 {
		return ""
	}
	after := output[start+len(begin):]
	section, _, _ := strings.Cut(after, end)
	return section
}

func (s WorkspaceRsync) stageProducers(ctx context.Context, plans []domain.DataTransferPlan, root string, runs []domain.DataTransferRun) ([]string, []rsyncStats, error) {
	stages := make([]string, len(plans))
	ingress := make([]rsyncStats, len(plans))
	seen := make(map[string][32]byte)
	for i, plan := range plans {
		if plan.Destination.URI != plans[0].Destination.URI || plan.Destination.Path != plans[0].Destination.Path {
			return nil, nil, fmt.Errorf("workspace transfers for a successor must share a destination")
		}
		source, err := s.Resolver.ResolveTransferEndpoint(ctx, plan.Source)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve producer %q: %w", plan.ProducerActivityID, err)
		}
		stage := filepath.Join(root, strconv.Itoa(i))
		if err := os.Mkdir(stage, 0700); err != nil {
			return nil, nil, err
		}
		stages[i] = stage
		stats, err := syncDirectory(ctx, source, domain.TransferEndpoint{URI: (&url.URL{Scheme: "file", Path: stage}).String()}, false)
		recordWorkspaceRsyncStats(&runs[i], stats, strings.HasPrefix(source.URI, "ssh://"))
		if err != nil {
			return nil, nil, fmt.Errorf("stage producer %q: %w", plan.ProducerActivityID, err)
		}
		ingress[i] = stats
		if err := inspectWorkspace(stage, seen); err != nil {
			return nil, nil, fmt.Errorf("producer %q: %w", plan.ProducerActivityID, err)
		}
	}
	return stages, ingress, nil
}

func publishWorkspace(ctx context.Context, plans []domain.DataTransferPlan, stages []string, ingress []rsyncStats, destination domain.TransferEndpoint, runs []domain.DataTransferRun) error {
	for i, stage := range stages {
		source := domain.TransferEndpoint{URI: (&url.URL{Scheme: "file", Path: stage}).String()}
		if err := checkDestinationConflicts(ctx, source, destination); err != nil {
			return fmt.Errorf("producer %q: %w", plans[i].ProducerActivityID, err)
		}
		stats, copyErr := syncDirectory(ctx, source, destination, true)
		recordWorkspaceRsyncStats(&runs[i], stats, strings.HasPrefix(destination.URI, "ssh://"))
		if copyErr != nil {
			return fmt.Errorf("publish producer %q: %w", plans[i].ProducerActivityID, copyErr)
		}
		runs[i].TransferredBytes = stats.fileBytes
		runs[i].LogicalBytes = ingress[i].fileBytes
		runs[i].FilesTransferred = stats.files
		if runs[i].NetworkBytes > 0 {
			// The source sent these bytes to the gateway even if a destination
			// already had the same content. Count actual traffic, not just new files.
			runs[i].TransferredBytes = ingress[i].fileBytes
			runs[i].FilesTransferred = ingress[i].files
		}
		runs[i].Route.FilesTransferred = runs[i].FilesTransferred
		runs[i].FinishedAt = float64(time.Now().UnixNano()) / 1e9
		runs[i].DurationSeconds = max(0, runs[i].FinishedAt-runs[i].StartedAt)
		runs[i].Status = domain.TransferCompleted
	}
	return nil
}

func inspectWorkspace(root string, seen map[string][32]byte) error {
	return filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == root || entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("workspace contains unsupported entry %q (symlinks are not transferred)", name)
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, file)
		_ = file.Close()
		if err != nil {
			return err
		}
		var digest [32]byte
		copy(digest[:], h.Sum(nil))
		if old, ok := seen[rel]; ok && old != digest {
			return fmt.Errorf("conflicting contents for workspace path %q across predecessors", rel)
		}
		seen[rel] = digest
		return nil
	})
}

type rsyncStats struct {
	bytes, fileBytes, files int64
	retries                 int
	lastRetryError          string
	networkBytesIncomplete  bool
	channelWaitSeconds      float64
	executionSeconds        float64
}

func syncDirectory(ctx context.Context, source, destination domain.TransferEndpoint, ignoreExisting bool) (rsyncStats, error) {
	args, err := rsyncArgs(source, destination)
	if err != nil {
		return rsyncStats{}, err
	}
	if ignoreExisting {
		args = append([]string{"--ignore-existing"}, args...)
	}
	waitStarted := time.Now()
	release, err := acquireRsyncSSHChannel(ctx, source, destination)
	waitSeconds := time.Since(waitStarted).Seconds()
	if err != nil {
		return rsyncStats{channelWaitSeconds: waitSeconds}, err
	}
	defer release()
	var executionSeconds float64
	stats, err := retryWorkspaceRsync(ctx, time.Second, strings.HasPrefix(destination.URI, "ssh://"), func() ([]byte, error) {
		started := time.Now()
		output, copyErr := exec.CommandContext(ctx, "rsync", args...).CombinedOutput()
		executionSeconds += time.Since(started).Seconds()
		return output, copyErr
	})
	stats.channelWaitSeconds = waitSeconds
	stats.executionSeconds = executionSeconds
	return stats, err
}

func rsyncArgs(source, destination domain.TransferEndpoint) ([]string, error) {
	from, err := rsyncLocation(source)
	if err != nil {
		return nil, err
	}
	to, err := rsyncLocation(destination)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(source.URI, "ssh://") && strings.HasPrefix(destination.URI, "ssh://") {
		return nil, fmt.Errorf("rsync requires one local staging endpoint")
	}
	remote := source
	if strings.HasPrefix(destination.URI, "ssh://") {
		remote = destination
	}
	args := []string{"-r", "--links", "--checksum", "--stats", "--out-format=%i %n", "--no-perms", "--no-owner", "--no-group", "--no-times"}
	if strings.HasPrefix(remote.URI, "ssh://") {
		// Send remote paths through the rsync protocol instead of the remote
		// shell. Newer rsync versions otherwise treat shell quotes as literal
		// characters in the path.
		args = append(args, "--secluded-args")
		ssh := []string{"ssh"}
		ssh = append(ssh, sshArgs(remote)...)
		for i := range ssh {
			ssh[i] = rsyncRemoteShellArgument(ssh[i])
		}
		args = append(args, "-e", strings.Join(ssh, " "))
	}
	return append(args, from, to), nil
}

// rsync parses the value passed to -e itself. Quoting every token with shell()
// breaks nested ProxyCommand values because those values already contain
// single-quoted paths. Keep ordinary SSH arguments untouched and quote only
// the values that rsync must preserve as one argument.
func rsyncRemoteShellArgument(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n\"\\") {
		return value
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func rsyncLocation(endpoint domain.TransferEndpoint) (string, error) {
	u, err := url.Parse(endpoint.URI)
	if err != nil {
		return "", err
	}
	if u.Scheme == "file" && filepath.IsAbs(u.Path) {
		return filepath.Clean(u.Path) + "/", nil
	}
	if u.Scheme == "ssh" {
		host, path, err := sshTarget(endpoint, "")
		if err != nil {
			return "", err
		}
		if strings.ContainsAny(host, " \t\r\n") || strings.ContainsAny(path, "\r\n") {
			return "", fmt.Errorf("invalid rsync SSH location")
		}
		return host + ":" + path + "/", nil
	}
	return "", fmt.Errorf("rsync workspace endpoint %q is unsupported", u.Scheme)
}

func makeWorkspaceDirectory(ctx context.Context, endpoint domain.TransferEndpoint) error {
	u, err := url.Parse(endpoint.URI)
	if err != nil {
		return err
	}
	if u.Scheme == "file" {
		return os.MkdirAll(u.Path, 0750)
	}
	if u.Scheme != "ssh" {
		return fmt.Errorf("unsupported workspace destination %q", u.Scheme)
	}
	host, path, err := sshTarget(endpoint, "")
	if err != nil {
		return err
	}
	output, err := runSSHCombinedOutput(ctx, endpoint, append(sshArgs(endpoint), host, "mkdir -p -- "+shell(path)), nil)
	if err != nil {
		return fmt.Errorf("create remote workspace: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func rejectWorkspaceSymlinks(ctx context.Context, endpoint domain.TransferEndpoint) error {
	return rejectWorkspaceSymlinksWithWait(ctx, endpoint, nil)
}

func rejectWorkspaceSymlinksWithWait(ctx context.Context, endpoint domain.TransferEndpoint, channelWait *float64) error {
	u, err := url.Parse(endpoint.URI)
	if err != nil {
		return err
	}
	if u.Scheme == "file" {
		return filepath.WalkDir(u.Path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("destination workspace contains symlink %q", path)
			}
			return nil
		})
	}
	if u.Scheme != "ssh" {
		return fmt.Errorf("unsupported workspace destination %q", u.Scheme)
	}
	host, path, err := sshTarget(endpoint, "")
	if err != nil {
		return err
	}
	waitStarted := time.Now()
	command, release, err := queuedSSHCommand(ctx, endpoint, append(sshArgs(endpoint), host, "find "+shell(path)+" -type l -print -quit"))
	if channelWait != nil {
		*channelWait = time.Since(waitStarted).Seconds()
	}
	if err != nil {
		return err
	}
	defer release()
	var stderr strings.Builder
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(string(output))
		}
		return fmt.Errorf("inspect remote workspace symlinks: %w: %s", err, message)
	}
	if name := strings.TrimSpace(string(output)); name != "" {
		return fmt.Errorf("destination workspace contains symlink %q", name)
	}
	return nil
}

func checkDestinationConflicts(ctx context.Context, source, destination domain.TransferEndpoint) error {
	args, err := rsyncArgs(source, destination)
	if err != nil {
		return err
	}
	release, err := acquireRsyncSSHChannel(ctx, source, destination)
	if err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "rsync", append([]string{"--dry-run"}, args...)...).CombinedOutput()
	release()
	if err != nil {
		return fmt.Errorf("inspect destination: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fullOutput := string(output)
	all := changedRsyncFiles(fullOutput)
	release, err = acquireRsyncSSHChannel(ctx, source, destination)
	if err != nil {
		return err
	}
	output, err = exec.CommandContext(ctx, "rsync", append([]string{"--dry-run", "--ignore-existing"}, args...)...).CombinedOutput()
	release()
	if err != nil {
		return fmt.Errorf("inspect existing destination: %w: %s", err, strings.TrimSpace(string(output)))
	}
	newFiles := changedRsyncFiles(string(output))
	for path := range all {
		if !newFiles[path] {
			return fmt.Errorf("destination already has different content at %q; no file was overwritten", path)
		}
	}
	return nil
}

func acquireRsyncSSHChannel(ctx context.Context, source, destination domain.TransferEndpoint) (func(), error) {
	remote := source
	if strings.HasPrefix(destination.URI, "ssh://") {
		remote = destination
	}
	if !strings.HasPrefix(remote.URI, "ssh://") {
		return func() {}, nil
	}
	return provider.AcquireSSHChannel(ctx, sshControlPath(sshArgs(remote)))
}

func changedRsyncFiles(output string) map[string]bool {
	files := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 && len(parts[0]) > 1 && (parts[0][0] == '>' || parts[0][0] == '<') && parts[0][1] == 'f' {
			files[parts[1]] = true
		}
	}
	return files
}

func parseRsyncStats(output string, sending bool) rsyncStats {
	stats := rsyncStats{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) == 0 {
			continue
		}
		value := strings.ReplaceAll(fields[0], ",", "")
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		switch parts[0] {
		case "Total bytes sent", "Total sent":
			if sending {
				stats.bytes = n
			}
		case "Total bytes received", "Total received":
			if !sending {
				stats.bytes = n
			}
		case "Total transferred file size":
			stats.fileBytes = n
		case "Number of regular files transferred", "Number of files transferred":
			stats.files = n
		}
	}
	return stats
}
