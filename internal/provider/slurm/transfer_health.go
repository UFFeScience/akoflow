package slurm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

// ArtifactLocationChecker validates an existing location without transferring
// bytes. Compute-node validation is opt-in because it consumes a scheduler
// allocation.
type ArtifactLocationChecker struct{ executor runtimecommon.CommandExecutor }

func NewArtifactLocationChecker(executors ...runtimecommon.CommandExecutor) *ArtifactLocationChecker {
	executor := runtimecommon.CommandExecutor(runtimecommon.OSCommandExecutor{})
	if len(executors) > 0 && executors[0] != nil {
		executor = executors[0]
	}
	return &ArtifactLocationChecker{executor: executor}
}

func (c *ArtifactLocationChecker) Check(ctx context.Context, connection domain.EnvironmentConnection, locationRef, path, expectedDigest string, expectedSize int64, probeComputeNode bool) (domain.ArtifactLocationHealth, error) {
	executor := c.executor
	if connection.Type == domain.ConnectionSSH {
		executor = runtimecommon.SSHCommandExecutor{Executor: c.executor, Endpoint: connection.Endpoint, Username: connection.Username, Port: configInt(connection.Configuration, "port"), IdentityFile: credentialFile(connection.CredentialRef)}
	}
	health := domain.ArtifactLocationHealth{LocationRef: locationRef, ExpectedDigest: expectedDigest, ExpectedSizeBytes: expectedSize, CheckedAt: time.Now().UTC()}
	script := `p="$1"; test -e "$p" || exit 20; test -r "$p" || exit 21; size=$(wc -c < "$p" | tr -d ' '); digest=$(sha256sum "$p" | awk '{print $1}'); printf '%s|%s\n' "$size" "$digest"`
	output, err := executor.Run(ctx, "/bin/sh", []string{"-c", script, "--", path}, nil)
	if err != nil {
		health.Reason = locationCheckReason(err)
		return health, nil
	}
	fields := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(fields) != 2 {
		health.Reason = "invalid location health output"
		return health, nil
	}
	fmt.Sscan(fields[0], &health.SizeBytes)
	health.Digest, health.Exists, health.LoginNodeReadable = fields[1], true, true
	if expectedSize > 0 && health.SizeBytes != expectedSize {
		health.Reason = "artifact size differs from expected"
		return health, nil
	}
	if expectedDigest != "" && !strings.EqualFold(strings.TrimPrefix(health.Digest, "sha256:"), strings.TrimPrefix(expectedDigest, "sha256:")) {
		health.Reason = "artifact digest differs from expected"
		return health, nil
	}
	if probeComputeNode {
		ok := false
		probe := `test -r "$1" && (apptainer inspect "$1" >/dev/null 2>&1 || singularity inspect "$1" >/dev/null 2>&1 || true)`
		if _, err := executor.Run(ctx, "srun", []string{"--nodes=1", "--ntasks=1", "--time=1", "/bin/sh", "-c", probe, "--", path}, nil); err == nil {
			ok = true
		}
		health.ComputeNodeReadable = &ok
		if !ok {
			health.Reason = "compute-node artifact probe failed"
		}
	}
	return health, nil
}

func locationCheckReason(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "exit status 20"):
		return "artifact does not exist"
	case strings.Contains(message, "exit status 21"):
		return "artifact is not readable on login node"
	default:
		return message
	}
}
