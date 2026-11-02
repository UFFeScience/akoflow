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
	oldSchema := schema.SQL
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
	oldSchema := schema.SQL
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
