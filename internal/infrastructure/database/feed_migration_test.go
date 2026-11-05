package database

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
)

func TestFeedIndexesMigrateExistingDatabaseWithoutLosingRuns(t *testing.T) {
	db := memoryDatabase(t)
	ctx := context.Background()
	oldSchema := schemaBeforeUnifiedAudit()
	for _, line := range strings.Split(runFeedIndexesSQL+catalogReadIndexesSQL, "\n") {
		if line != "" {
			oldSchema = strings.ReplaceAll(oldSchema, line+"\n", "")
		}
	}
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_metadata(checksum,applied_at) VALUES (?,CURRENT_TIMESTAMP)`, schemaWithWorkflowOperationEvents); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO workflow_definitions(id,external_id,name) VALUES ('preserved','preserved','Preserved')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Bootstrap(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM workflow_definitions WHERE id='preserved'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("existing records changed: %d, %v", count, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='data_transfer_observations_run_idx'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing index: %d, %v", count, err)
	}
}

func TestScopedReadIndexesUpgradeThePreviouslyIndexedCatalog(t *testing.T) {
	db := memoryDatabase(t)
	oldSchema := schemaBeforeUnifiedAudit()
	for _, line := range strings.Split(catalogReadIndexesSQL, "\n") {
		if line != "" {
			oldSchema = strings.ReplaceAll(oldSchema, line+"\n", "")
		}
	}
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_metadata(checksum,applied_at) VALUES (?,CURRENT_TIMESTAMP)`, schemaWithRunFeedIndexes); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO environments(id,name) VALUES ('preserved','Preserved')`); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM environments WHERE id='preserved'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("catalog records were lost: %d %v", count, err)
	}
}

func schemaBeforeUnifiedAudit() string {
	old := strings.ReplaceAll(schema.SQL, `CREATE TABLE audit_entity_context (
 entity_type TEXT NOT NULL,
 entity_id TEXT NOT NULL,
 provenance TEXT NOT NULL,
 PRIMARY KEY(entity_type, entity_id)
);
`, "")
	for _, line := range strings.Split(unifiedAuditSQL, "\n") {
		if strings.HasPrefix(line, "ALTER TABLE audit_events ADD COLUMN ") {
			definition := strings.TrimSuffix(strings.TrimPrefix(line, "ALTER TABLE audit_events ADD COLUMN "), ";")
			old = strings.ReplaceAll(old, " "+definition+",\n", "")
		} else if strings.HasPrefix(line, "CREATE INDEX ") {
			old = strings.ReplaceAll(old, line+"\n", "")
		}
	}
	return old
}

func TestUnifiedAuditMigrationPreservesRecords(t *testing.T) {
	db := memoryDatabase(t)
	if _, err := db.Exec(schemaBeforeUnifiedAudit()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO schema_metadata(checksum,applied_at) VALUES (?,CURRENT_TIMESTAMP)", "118459cfa1b2ace00d67b0e6f2a29e0f626525fab0a725221c66b6c814a5a313"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO audit_events(id,event_type,outcome,summary,occurred_at) VALUES ('preserved','old','succeeded','old',CURRENT_TIMESTAMP)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Bootstrap(context.Background(), db); err != nil {
			t.Fatal(err)
		}
	}
	var id string
	if err := db.QueryRow("SELECT id FROM audit_events").Scan(&id); err != nil || id != "preserved" {
		t.Fatalf("%s %v", id, err)
	}
}
