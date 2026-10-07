package transfer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Retries operate on the same staging directory. Rsync's checksum comparison
// reuses completed files; no activity command or publication gate is retried.
func retryWorkspaceRsync(ctx context.Context, delay time.Duration, sending bool, copy func() ([]byte, error)) (rsyncStats, error) {
	var total rsyncStats
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		output, err := copy()
		stats := parseRsyncStats(string(output), sending)
		total.bytes += stats.bytes
		if err == nil {
			total.fileBytes, total.files = stats.fileBytes, stats.files
			return total, nil
		}
		// A failed rsync may omit its final statistics. Keep reported received
		// bytes as a lower bound and explicitly flag incomplete accounting.
		if stats.bytes == 0 {
			total.bytes += interruptedRsyncBytes(string(output))
			total.networkBytesIncomplete = true
		}
		failure := fmt.Errorf("rsync: %w: %s", err, strings.TrimSpace(string(output)))
		if ctx.Err() != nil || !transientWorkspaceRsync(string(output)) || attempt == 2 {
			return total, failure
		}
		total.retries++
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		total.lastRetryError = lines[len(lines)-1]
		timer := time.NewTimer(delay * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return total, ctx.Err()
		case <-timer.C:
		}
	}
	return total, fmt.Errorf("workspace rsync retries exhausted")
}

func transientWorkspaceRsync(output string) bool {
	message := strings.ToLower(output)
	for _, permanent := range []string{"permission denied", "host key verification failed", "no such file", "content conflict", "no space left"} {
		if strings.Contains(message, permanent) {
			return false
		}
	}
	return strings.Contains(message, "connection unexpectedly closed") || strings.Contains(message, "connection reset") || strings.Contains(message, "connection closed") || strings.Contains(message, "broken pipe")
}

func interruptedRsyncBytes(output string) int64 {
	var received int64
	for _, line := range strings.Split(output, "\n") {
		_, suffix, ok := strings.Cut(line, "connection unexpectedly closed (")
		if !ok {
			continue
		}
		value, _, ok := strings.Cut(suffix, " bytes received so far)")
		if ok {
			n, _ := strconv.ParseInt(value, 10, 64)
			received = max(received, n)
		}
	}
	return received
}

func recordWorkspaceRsyncStats(run *domain.DataTransferRun, stats rsyncStats, network bool) {
	if network {
		run.NetworkBytes += stats.bytes
		run.Route.NetworkBytesIncomplete = run.Route.NetworkBytesIncomplete || stats.networkBytesIncomplete
	}
	run.Route.RetryCount += stats.retries
	if stats.lastRetryError != "" {
		run.Route.LastRetryError = stats.lastRetryError
	}
}
