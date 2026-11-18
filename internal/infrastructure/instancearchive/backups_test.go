package instancearchive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/instance"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func TestManagedBackupsRecordScheduleAndRetention(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "active.sqlite")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	service, err := New(db, path, filepath.Join(root, "instances"), "", "test")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "backups")
	status, err := service.ConfigureBackups(instance.BackupSettings{Destination: destination, IntervalHours: 1, Retain: 1})
	if err != nil || status.NextScheduled.IsZero() {
		t.Fatalf("configure backup: %+v, %v", status, err)
	}
	manual, err := service.CreateBackup(ctx)
	if err != nil || manual.Last == nil || manual.Last.Scheduled {
		t.Fatalf("manual backup: %+v, %v", manual, err)
	}
	foreign := filepath.Join(destination, "akoflow-backup-foreign.zip")
	if err := os.WriteFile(foreign, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	manual.NextScheduled = time.Now().Add(-time.Minute)
	if err := service.writeBackupStatus(manual); err != nil {
		t.Fatal(err)
	}
	scheduled, err := service.runBackup(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	last := scheduled.LastScheduled
	if last == nil || !last.Scheduled || len(last.Removed) != 1 || last.Removed[0] != manual.Last.File {
		t.Fatalf("unexpected scheduled result: %+v", last)
	}
	if _, err := os.Stat(filepath.Join(destination, last.File)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("retention touched another installation's files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, manual.Last.File)); !os.IsNotExist(err) {
		t.Fatalf("expired backup still present: %v", err)
	}
	persisted, err := service.BackupStatus()
	if err != nil || persisted.LastScheduled == nil || persisted.LastScheduled.File != last.File {
		t.Fatalf("persist backup status: %+v, %v", persisted, err)
	}
}

func TestBackupDuringUncommittedWritePreservesConsistentState(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "active.sqlite")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO environments (id, name) VALUES ('environment', 'Committed name')"); err != nil {
		t.Fatal(err)
	}
	writer, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE environments SET name = 'Uncommitted name'"); err != nil {
		t.Fatal(err)
	}
	service, err := New(db, path, filepath.Join(root, "instances"), "", "test")
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.CreateBackup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	item, err := service.ImportManagedBackup(ctx, status.Last.File)
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(service.root, item.ID, "database", "akoflow.sqlite")
	if got := restoredEnvironmentName(t, archived); got != "Committed name" {
		t.Fatalf("snapshot contains %q", got)
	}
	if got := restoredEnvironmentName(t, path); got != "Uncommitted name" {
		t.Fatalf("active instance contains %q", got)
	}
}
