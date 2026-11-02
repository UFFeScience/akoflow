package execution

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ReleasePreservedWorkspaces records durable export and releases leases in one
// transaction. Cancellation and absence of live handles are rechecked here.
func (r *Repository) ReleasePreservedWorkspaces(ctx context.Context, runID, instanceID string, ids []string, archive string) error {
	if len(ids) == 0 || !strings.HasPrefix(archive, "storage/preserved-workspaces/") || !strings.Contains(archive, " sha256:") {
		return fmt.Errorf("verified archive required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM execution_runs WHERE id=?`, runID).Scan(&status); err != nil {
		return err
	}
	if status != "cancelled" {
		return fmt.Errorf("run is not cancelled")
	}
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_handles WHERE execution_run_id=? AND status NOT IN ('completed','failed','stopped')`, runID).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return fmt.Errorf("run has active handles")
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM transfer_runs WHERE execution_run_id=? AND status NOT IN ('completed','failed','cancelled','stopped')`, runID).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return fmt.Errorf("run has active transfers")
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cloud_operation_runs WHERE instance_id=? AND status IN ('queued','running')`, instanceID).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return fmt.Errorf("cloud generation has active infrastructure operations")
	}
	now := time.Now().UTC()
	reason := "cancelled consumers; verified relay preservation: " + archive
	for _, id := range ids {
		var uri string
		var pinned bool
		if err := tx.QueryRowContext(ctx, `SELECT uri,pinned FROM activity_workspaces WHERE id=? AND execution_run_id=?`, id, runID).Scan(&uri, &pinned); err != nil {
			return err
		}
		location, err := url.Parse(uri)
		if err != nil || pinned || location.Query().Get("cloudInstanceId") != instanceID {
			return fmt.Errorf("workspace identity or retention changed")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE workspace_leases SET released=1,released_at=?,release_reason=? WHERE workspace_id=? AND execution_run_id=? AND released=0`, now, reason, id, runID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE activity_workspaces SET state='released',released_at=?,release_reason=? WHERE id=? AND execution_run_id=?`, now, reason, id, runID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
