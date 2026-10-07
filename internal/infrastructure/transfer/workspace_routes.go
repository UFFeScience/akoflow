package transfer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// sharedWorkspaceProof verifies access and filesystem identity at execution
// time. A matching SSH connection alone is not evidence of shared Lustre.
func sharedWorkspaceProof(ctx context.Context, source, destination domain.TransferEndpoint) (string, bool) {
	if !sameSSHWorkspaceEndpoint(source, destination) {
		return "", false
	}
	host, target, err := sshTarget(destination, "")
	if err != nil {
		return "", false
	}
	_, origin, err := sshTarget(source, "")
	if err != nil {
		return "", false
	}
	command := "test -d " + shell(origin) + " && test -r " + shell(origin) +
		" && stat -f -c '%T %i' -- " + shell(origin) + " " + shell(target)
	output, err := runSSHCombinedOutput(ctx, destination, append(sshArgs(destination), host, command), nil)
	if err != nil {
		return "", false
	}
	return parseSharedWorkspaceProof(string(output))
}

func parseSharedWorkspaceProof(output string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 || lines[0] != lines[1] {
		return "", false
	}
	fields := strings.Fields(lines[0])
	if len(fields) != 2 || fields[0] != "lustre" || fields[1] == "" {
		return "", false
	}
	return lines[0], true
}

func (s WorkspaceRsync) syncDependencyRoutes(ctx context.Context, plans []domain.DataTransferPlan, sources []domain.TransferEndpoint, destination domain.TransferEndpoint, runs []domain.DataTransferRun) ([]domain.DataTransferRun, error) {
	if err := makeWorkspaceDirectory(ctx, destination); err != nil {
		return runs, err
	}
	host, target, err := sshTarget(destination, "")
	if err != nil {
		return runs, err
	}
	prepared := append([]domain.TransferEndpoint(nil), sources...)
	proofs := make([]string, len(plans))
	network := make([]rsyncStats, len(plans))
	var remoteRoot string
	defer func() {
		if remoteRoot != "" {
			// Only the exact mktemp-generated preparation directory is removed.
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			_, _ = runSSHCombinedOutput(cleanupCtx, destination, append(sshArgs(destination), host, "rm -rf -- "+shell(remoteRoot)), nil)
		}
	}()
	for i, source := range sources {
		if proof, ok := sharedWorkspaceProof(ctx, source, destination); ok {
			proofs[i] = proof
			runs[i].Strategy, runs[i].Route.Strategy = domain.TransferDirectRuntime, domain.TransferDirectRuntime
			runs[i].Route.Reason = "verified shared filesystem: " + proof
			continue
		}
		runs[i].Route.Reason = "per-dependency staging fallback; shared storage access not proven"
		if strings.HasPrefix(source.URI, "file://") {
			runs[i].Strategy, runs[i].Route.Strategy = domain.TransferDirectRuntime, domain.TransferDirectRuntime
			runs[i].Route.Reason = "network rsync from local producer to remote merge preparation"
		}
		if remoteRoot == "" {
			output, createErr := runSSHCombinedOutput(ctx, destination, append(sshArgs(destination), host,
				"mktemp -d -- "+shell(filepath.Join(filepath.Dir(target), ".akoflow-merge-XXXXXX"))), nil)
			if createErr != nil {
				return runs, fmt.Errorf("prepare remote merge: %w", createErr)
			}
			remoteRoot = strings.TrimSpace(string(output))
			if !filepath.IsAbs(remoteRoot) || filepath.Dir(remoteRoot) != filepath.Dir(target) || !strings.HasPrefix(filepath.Base(remoteRoot), ".akoflow-merge-") {
				remoteRoot = ""
				return runs, fmt.Errorf("invalid remote preparation directory")
			}
		}
		remoteURL, _ := url.Parse(destination.URI)
		remoteURL.Path = filepath.Join(remoteRoot, strconv.Itoa(i))
		prepared[i] = destination
		prepared[i].URI = remoteURL.String()
		if err := makeWorkspaceDirectory(ctx, prepared[i]); err != nil {
			return runs, err
		}
		copySource := source
		if !strings.HasPrefix(source.URI, "file://") {
			localRoot, err := os.MkdirTemp("", "akoflow-dependency-")
			if err != nil {
				return runs, err
			}
			defer os.RemoveAll(localRoot)
			copySource = domain.TransferEndpoint{URI: (&url.URL{Scheme: "file", Path: localRoot}).String()}
			stats, err := syncDirectory(ctx, source, copySource, false)
			if err != nil {
				return runs, err
			}
			network[i].bytes += stats.bytes
			runs[i].NetworkBytes = network[i].bytes
		}
		stats, err := syncDirectory(ctx, copySource, prepared[i], false)
		if err != nil {
			return runs, err
		}
		network[i].bytes += stats.bytes
		network[i].fileBytes = stats.fileBytes
		runs[i].NetworkBytes = network[i].bytes
	}
	// Validate all paths/content together before any successor publication.
	result, err := syncRemoteWorkspaceDirect(ctx, plans, prepared, destination, runs)
	for i := range result {
		result[i].NetworkBytes = network[i].bytes
		if proofs[i] != "" {
			result[i].Route.Reason = "verified shared filesystem: " + proofs[i]
		} else {
			result[i].Strategy, result[i].Route.Strategy = domain.TransferGateway, domain.TransferGateway
			result[i].Route.Reason = "per-dependency staging fallback; shared storage access not proven"
			if strings.HasPrefix(sources[i].URI, "file://") {
				result[i].Strategy, result[i].Route.Strategy = domain.TransferDirectRuntime, domain.TransferDirectRuntime
				result[i].Route.Reason = "network rsync from local producer to remote merge preparation"
			}
		}
	}
	return result, err
}
