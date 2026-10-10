package cloud_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbcloud "github.com/UFFeScience/akoflow/internal/infrastructure/database/cloud"
)

func TestLifecycleSupersededAttemptStillProtectsLiveWork(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "guard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
	CREATE TABLE task_executions(execution_run_id TEXT, activity_id TEXT, attempt INTEGER, status TEXT, metadata TEXT);
	CREATE TABLE activity_handles(resource_id TEXT, status TEXT, metadata TEXT);
	CREATE TABLE cloud_provisioned_instances(id TEXT, capacity_target_id TEXT);
	CREATE TABLE transfer_runs(status TEXT, route TEXT);
	CREATE TABLE artifact_materializations(run_id TEXT, resource_id TEXT, status TEXT);
	CREATE TABLE cloud_operation_runs(id TEXT, instance_id TEXT, execution_run_id TEXT, status TEXT);
	CREATE TABLE console_sessions(resource_id TEXT, status TEXT);
	CREATE TABLE activity_workspaces(id TEXT, state TEXT, uri TEXT);
	CREATE TABLE workspace_leases(workspace_id TEXT, released INTEGER);
	INSERT INTO cloud_provisioned_instances VALUES('vm','target');
	INSERT INTO task_executions VALUES('run','activity',1,'running','{"cloudInstanceId":"vm"}');`)
	if err != nil {
		t.Fatal(err)
	}
	r := dbcloud.New(db)
	ctx := context.Background()
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err == nil {
		t.Fatal("live task must block cleanup")
	}
	if _, err := db.Exec(`INSERT INTO task_executions VALUES('run','activity',2,'completed','{"cloudInstanceId":"vm"}')`); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err != nil {
		t.Fatalf("completed replacement: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO activity_workspaces VALUES('producer','sealed','cloud://target/path?cloudInstanceId=vm&runtime=x');
		INSERT INTO workspace_leases VALUES('producer',0)`); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err == nil {
		t.Fatal("completed producer with pending consumers must block cleanup")
	}
	if _, err := db.Exec(`UPDATE workspace_leases SET released=1`); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err != nil {
		t.Fatalf("released consumer lease: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO activity_handles VALUES('target','running','{"cloudInstanceId":"old-vm"}')`); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err != nil {
		t.Fatalf("another instance's handle must not block cleanup: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO activity_handles VALUES('target','running','{"cloudInstanceId":"vm"}')`); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err == nil {
		t.Fatal("live handle must block even with completed replacement")
	}
	if _, err := db.Exec(`UPDATE activity_handles SET status='stopped'; INSERT INTO transfer_runs VALUES('running','{"sourceCloudInstanceId":"vm"}')`); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err == nil {
		t.Fatal("active transfer must block cleanup")
	}
	if _, err := db.Exec(`UPDATE transfer_runs SET status='completed'; INSERT INTO task_executions VALUES('run','activity',3,'preparing','{"cloudInstanceId":"vm"}')`); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckCloudLifecycle(ctx, "vm", "destroy"); err == nil {
		t.Fatal("newer preparation must still block cleanup")
	}
}
