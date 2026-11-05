package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func TestReadOnlyArchiveUsesSeparateWritableAccessAudit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AKOFLOW_INSTANCE_ARCHIVE_ROOT", root)
	archive := filepath.Join(root, "snapshot", "database", "akoflow.sqlite")
	db, err := database.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := os.WriteFile(filepath.Join(root, ".active-instance"), []byte("snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	storage, err := openPersistence(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.database.Close()
	defer storage.analytics.Close()
	if storage.auditDatabase == nil || !storage.readOnly {
		t.Fatal("missing separate audit persistence")
	}
	defer storage.auditDatabase.Close()
	if err := storage.audit.RecordAuditEvent(context.Background(), domainaudit.Event{ID: "request", EventType: "api.access", Outcome: domainaudit.OutcomeSucceeded, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	items, err := storage.audit.ListAuditEvents(context.Background(), domainaudit.Filter{})
	if err != nil || len(items) != 1 {
		t.Fatalf("%+v %v", items, err)
	}
	var count int
	if err := storage.database.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&count); err != nil || count != 0 {
		t.Fatalf("snapshot was modified: count=%d err=%v", count, err)
	}
}
