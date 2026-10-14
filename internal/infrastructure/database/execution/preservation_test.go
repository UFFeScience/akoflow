package execution

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func TestPreservedWorkspaceReleaseRequiresCancelledIdleExactGeneration(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE execution_runs(id TEXT,status TEXT);
CREATE TABLE activity_handles(execution_run_id TEXT,status TEXT);
CREATE TABLE transfer_runs(execution_run_id TEXT,status TEXT);
CREATE TABLE cloud_operation_runs(instance_id TEXT,status TEXT);
CREATE TABLE activity_workspaces(id TEXT,execution_run_id TEXT,uri TEXT,pinned BOOLEAN,state TEXT,released_at DATETIME,release_reason TEXT);
CREATE TABLE workspace_leases(workspace_id TEXT,execution_run_id TEXT,released BOOLEAN,released_at DATETIME,release_reason TEXT);
INSERT INTO execution_runs VALUES('run','running');
INSERT INTO activity_workspaces VALUES('workspace','run','ssh://worker/path?cloudInstanceId=vm',0,'sealed',NULL,'');
INSERT INTO workspace_leases VALUES('workspace','run',0,NULL,'');`)
	if err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	ctx := context.Background()
	archive := "storage/preserved-workspaces/digest.tar.gz sha256:digest"
	if err := repo.ReleasePreservedWorkspaces(ctx, "run", "vm", []string{"workspace"}, archive); err == nil {
		t.Fatal("live run released")
	}
	if _, err := db.Exec(`UPDATE execution_runs SET status='cancelled'; INSERT INTO activity_handles VALUES('run','running')`); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReleasePreservedWorkspaces(ctx, "run", "vm", []string{"workspace"}, archive); err == nil {
		t.Fatal("live handles released")
	}
	if _, err := db.Exec(`UPDATE activity_handles SET status='stopped'`); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReleasePreservedWorkspaces(ctx, "run", "another", []string{"workspace"}, archive); err == nil {
		t.Fatal("different generation released")
	}
	if err := repo.ReleasePreservedWorkspaces(ctx, "run", "vm", []string{"workspace", "missing"}, archive); err == nil {
		t.Fatal("partial identity set accepted")
	}
	var released bool
	if err := db.QueryRow(`SELECT released FROM workspace_leases`).Scan(&released); err != nil || released {
		t.Fatalf("transaction did not roll back: %v", err)
	}
	if err := repo.ReleasePreservedWorkspaces(ctx, "run", "vm", []string{"workspace"}, archive); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT released FROM workspace_leases`).Scan(&released); err != nil || !released {
		t.Fatalf("preserved lease not released: %v", err)
	}
}
