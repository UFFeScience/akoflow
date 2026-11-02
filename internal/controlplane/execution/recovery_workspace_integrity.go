package execution

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// A sealed catalog entry does not prove its files still exist after cleanup or
// an interrupted recovery. Missing data requires recomputing the producer.
func recoveryWorkspacePresent(workspace domain.ActivityWorkspace, usage domain.WorkspaceUsage) bool {
	return workspace.State == domain.WorkspaceSealed && usage.FileCount == workspace.FileCount && usage.SizeBytes == workspace.SizeBytes
}

// A regenerated producer must not be reclaimed using leases released by a
// previous attempt before its pending consumers have received the data again.
func restoreRecoveryWorkspaceLeases(ctx context.Context, store ports.WorkspaceStore, runID string, reusable map[string]bool) error {
	leases, err := store.ListWorkspaceLeases(ctx, runID)
	if err != nil {
		return err
	}
	for _, lease := range leases {
		if reusable[lease.ConsumerActivityID] || !lease.Released {
			continue
		}
		lease.Released, lease.ReleasedAt, lease.ReleaseReason = false, nil, ""
		if err := store.SaveWorkspaceLease(ctx, lease); err != nil {
			return fmt.Errorf("restore recovery workspace lease %q: %w", lease.ID, err)
		}
	}
	return nil
}
