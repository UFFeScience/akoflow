package transfer

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// A same-instance route requires independently resolved generation trust, not
// merely equal public addresses or a shared capacity target.
func sameVerifiedCloudWorkspaceEndpoint(source, destination domain.TransferEndpoint) bool {
	if source.CloudInstanceID == "" || source.CloudInstanceID != destination.CloudInstanceID || !sameSSHWorkspaceEndpoint(source, destination) {
		return false
	}
	a, errA := url.Parse(source.URI)
	b, errB := url.Parse(destination.URI)
	if errA != nil || errB != nil {
		return false
	}
	x, y := a.Query(), b.Query()
	return x.Get("strictHostKeyChecking") == "true" && y.Get("strictHostKeyChecking") == "true" &&
		x.Get("hostKeyAlias") != "" && x.Get("hostKeyAlias") == y.Get("hostKeyAlias") &&
		x.Get("knownHostsFile") != "" && x.Get("knownHostsFile") == y.Get("knownHostsFile")
}

func sameCloudWorkspaceProof(ctx context.Context, source, destination domain.TransferEndpoint) (string, bool) {
	if !sameVerifiedCloudWorkspaceEndpoint(source, destination) {
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
	output, err := runSSHCombinedOutput(ctx, destination, append(sshArgs(destination), host, sharedWorkspaceProofCommand(origin, target)), nil)
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 || lines[0] != lines[1] {
		return "", false
	}
	fields := strings.Fields(lines[0])
	if len(fields) != 5 || fields[1] == "" || fields[3] == "" || !strings.HasPrefix(fields[4], "/") {
		return "", false
	}
	return fields[0] + " " + strings.Join(fields[1:], "|"), true
}

// sharedWorkspaceProof verifies access and filesystem identity at execution
// time. A matching SSH connection or a zero filesystem ID alone is not proof
// of shared storage: the mounted source and mount path must also match.
func sharedWorkspaceProof(ctx context.Context, source, destination domain.TransferEndpoint) (string, bool) {
	proof, ok, _ := sharedWorkspaceProofDiagnostic(ctx, source, destination)
	return proof, ok
}

// Never persist command output, addresses, or paths in proof failure metadata.
func sharedWorkspaceProofDiagnostic(ctx context.Context, source, destination domain.TransferEndpoint) (string, bool, string) {
	if !sameSSHWorkspaceEndpoint(source, destination) {
		return "", false, "different_ssh_endpoints"
	}
	host, target, err := sshTarget(destination, "")
	if err != nil {
		return "", false, "invalid_target_endpoint"
	}
	_, origin, err := sshTarget(source, "")
	if err != nil {
		return "", false, "invalid_source_endpoint"
	}
	command := sharedWorkspaceProofCommand(origin, target)
	output, err := runSSHCombinedOutput(ctx, destination, append(sshArgs(destination), host, command), nil)
	if err != nil {
		return "", false, "proof_command_failed"
	}
	proof, ok := parseSharedWorkspaceProof(string(output))
	if !ok {
		return "", false, "filesystem_identity_or_format_rejected"
	}
	return proof, true, ""
}

func sharedWorkspaceProofCommand(origin, target string) string {
	return "set -eu; test -d " + shell(origin) + "; test -r " + shell(origin) +
		"; test -x " + shell(origin) + "; test -d " + shell(target) + "; test -w " + shell(target) +
		"; for workspace in " + shell(origin) + " " + shell(target) + "; do " +
		"stat -f -c '%T %i' -- \"$workspace\" | tr '\\n' ' '; " +
		"findmnt -rn -T \"$workspace\" -o FSTYPE,SOURCE,TARGET; done"
}

func parseSharedWorkspaceProof(output string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 || lines[0] != lines[1] {
		return "", false
	}
	fields := strings.Fields(lines[0])
	if len(fields) != 5 || fields[1] == "" || fields[3] == "" || !strings.HasPrefix(fields[4], "/") {
		return "", false
	}
	compatible := false
	switch fields[0] {
	case "lustre":
		compatible = fields[2] == "lustre"
	case "nfs", "nfs4":
		compatible = fields[2] == "nfs" || fields[2] == "nfs4"
	case "beegfs", "fhgfs":
		compatible = fields[2] == "beegfs" || fields[2] == "fhgfs"
	}
	if !compatible {
		return "", false
	}
	return fields[0] + " " + strings.Join(fields[1:], "|"), true
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
	localCloud := make([]bool, len(plans))
	privatePeers := make([]bool, len(plans))
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
		runs[i].Route.SourceAddress = workspaceRouteAddress(source.URI)
		runs[i].Route.TargetAddress = workspaceRouteAddress(destination.URI)
		proof, ok := sameCloudWorkspaceProof(ctx, source, destination)
		localCloud[i] = ok
		if !ok {
			proof, ok, runs[i].Route.SharedStorageProofFailure = sharedWorkspaceProofDiagnostic(ctx, source, destination)
		}
		if ok {
			proofs[i] = proof
			identity := strings.Fields(proof)
			runs[i].Route.SharedFilesystem = identity[0]
			runs[i].Route.SharedFilesystemID = identity[1]
			runs[i].Strategy, runs[i].Route.Strategy = domain.TransferDirectRuntime, domain.TransferDirectRuntime
			runs[i].Route.Reason = "verified shared filesystem: " + proof
			if localCloud[i] {
				runs[i].Strategy, runs[i].Route.Strategy = domain.TransferRuntimeLocal, domain.TransferRuntimeLocal
				runs[i].Route.Reason = "verified same concrete cloud instance and accessible workspace filesystem: " + proof
			}
			continue
		}
		runs[i].Route.Reason = "per-dependency staging fallback; shared storage access not proven"
		if strings.HasPrefix(source.URI, "file://") {
			runs[i].Strategy, runs[i].Route.Strategy = domain.TransferDirectRuntime, domain.TransferDirectRuntime
			runs[i].Route.Reason = "network rsync from local producer to remote merge preparation"
		}
		if remoteRoot == "" {
			remoteRoot, err = createRemoteMergeDirectory(ctx, destination, host, target)
			if err != nil {
				return runs, err
			}
		}
		remoteURL, _ := url.Parse(destination.URI)
		remoteURL.Path = filepath.Join(remoteRoot, strconv.Itoa(i))
		prepared[i] = destination
		prepared[i].URI = remoteURL.String()
	}
	// Separate staging directories allow concurrent preparation without concurrent
	// writes into the consumer workspace. Cancellation waits for every worker.
	barrierStart := time.Now()
	if err := parallelWorkspacePreparation(ctx, len(sources), s.Parallelism, func(ctx context.Context, i int) error {
		if proofs[i] != "" {
			return nil
		}
		peer, err := prepareDependencyArchive(ctx, sources[i], &prepared[i], &runs[i])
		privatePeers[i] = peer
		network[i].bytes = runs[i].NetworkBytes
		return err
	}); err != nil {
		return runs, err
	}
	barrierEnd := time.Now()
	recordSharedPreparationBarrier(runs, barrierStart, barrierEnd)
	// Validate all paths/content together before any successor publication.
	result, err := syncRemoteWorkspaceDirect(ctx, plans, prepared, destination, runs)
	annotateDependencyRoutes(sources, proofs, localCloud, privatePeers, network, result)
	return result, err
}

func createRemoteMergeDirectory(ctx context.Context, destination domain.TransferEndpoint, host, target string) (string, error) {
	output, err := runSSHCombinedOutput(ctx, destination, append(sshArgs(destination), host,
		"mktemp -d -- "+shell(filepath.Join(filepath.Dir(target), ".akoflow-merge-XXXXXX"))), nil)
	if err != nil {
		return "", fmt.Errorf("prepare remote merge: %w", err)
	}
	root := strings.TrimSpace(string(output))
	if !filepath.IsAbs(root) || filepath.Dir(root) != filepath.Dir(target) || !strings.HasPrefix(filepath.Base(root), ".akoflow-merge-") {
		return "", fmt.Errorf("invalid remote preparation directory")
	}
	return root, nil
}

func recordSharedPreparationBarrier(runs []domain.DataTransferRun, start, end time.Time) {
	for i := range runs {
		runs[i].Route.Phases = append(runs[i].Route.Phases, domain.TransferPhase{
			Name: "shared-preparation-barrier", StartedAt: float64(start.UnixNano()) / 1e9,
			FinishedAt: float64(end.UnixNano()) / 1e9, DurationSeconds: end.Sub(start).Seconds(),
		})
	}
}

// Route evidence must never serialize SSH credentials or URI query options.
func workspaceRouteAddress(raw string) string {
	address, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	address.User = nil
	address.RawQuery = ""
	address.Fragment = ""
	return address.String()
}

func prepareDependencyArchive(ctx context.Context, source domain.TransferEndpoint, prepared *domain.TransferEndpoint, run *domain.DataTransferRun) (bool, error) {
	peer := false
	start := float64(time.Now().UnixNano()) / 1e9
	(*run).StartedAt = start
	(*run).Route.PreparationStartedAt = start
	(*run).Route.Packaging = "tar-gzip"
	usedPeer, err := preparePeerWorkspaceArchive(ctx, source, (*prepared), &(*run))
	if usedPeer && err == nil {
		peer = true
		preparedURL, _ := url.Parse((*prepared).URI)
		preparedURL.Path = filepath.Join(preparedURL.Path, "contents")
		(*prepared).URI = preparedURL.String()
	} else if !usedPeer && err == nil {
		err = relayWorkspaceArchive(ctx, source, (*prepared), &(*run))
	}
	end := float64(time.Now().UnixNano()) / 1e9
	(*run).Route.PreparationFinishedAt = end
	(*run).Route.PreparationSeconds = end - start
	return peer, err
}

func annotateDependencyRoutes(sources []domain.TransferEndpoint, proofs []string, localCloud, privatePeers []bool, network []rsyncStats, result []domain.DataTransferRun) {
	for i := range result {
		result[i].NetworkBytes = network[i].bytes
		result[i].Route.FilesTransferred = result[i].FilesTransferred
		serverBytes := network[i].bytes
		if proofs[i] != "" || privatePeers[i] {
			serverBytes = 0
		}
		result[i].Route.ServerNetworkBytes = &serverBytes
		if proofs[i] != "" {
			result[i].Route.Reason = "verified shared filesystem: " + proofs[i]
			if localCloud[i] {
				result[i].Strategy, result[i].Route.Strategy = domain.TransferRuntimeLocal, domain.TransferRuntimeLocal
				result[i].Route.Reason = "verified same concrete cloud instance and accessible workspace filesystem: " + proofs[i]
			}
		} else {
			result[i].Strategy, result[i].Route.Strategy = domain.TransferGateway, domain.TransferGateway
			result[i].Route.Reason = "per-dependency staging fallback; shared storage access not proven"
			if privatePeers[i] {
				result[i].Strategy, result[i].Route.Strategy = domain.TransferDirectRuntime, domain.TransferDirectRuntime
				result[i].Route.NetworkDomain = sources[i].NetworkDomain
				result[i].Route.Reason = "verified private cloud peer generation and authenticated whole-workspace archive transfer"
			}
			if strings.HasPrefix(sources[i].URI, "file://") {
				result[i].Strategy, result[i].Route.Strategy = domain.TransferDirectRuntime, domain.TransferDirectRuntime
				result[i].Route.Reason = "network rsync from local producer to remote merge preparation"
			}
		}
	}
}
