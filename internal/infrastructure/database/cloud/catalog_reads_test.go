package cloud_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbcloud "github.com/UFFeScience/akoflow/internal/infrastructure/database/cloud"
)

func TestCloudOperationsFilterBeforeLimitWithStableOrdering(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cloud.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO environments(id,name) VALUES ('env','Selected'),('other','Other')`); err != nil {
		t.Fatal(err)
	}
	r := dbcloud.New(db)
	for _, op := range []domain.CloudOperationRun{
		{ID: "a", Kind: "provision", Status: "completed", InstanceID: "selected", EnvironmentID: "env", CreatedAt: time.Now().UTC()},
		{ID: "b", Kind: "provision", Status: "completed", InstanceID: "selected", EnvironmentID: "env", CreatedAt: time.Now().UTC()},
		{ID: "z", Kind: "provision", Status: "running", InstanceID: "unrelated", EnvironmentID: "other", CreatedAt: time.Now().UTC()},
	} {
		if err := r.CreateCloudOperation(ctx, op); err != nil {
			t.Fatal(err)
		}
	}
	filtered, err := r.ListCloudOperationsFiltered(ctx, domain.CloudOperationFilter{InstanceID: "selected", Limit: 1})
	if err != nil || len(filtered) != 1 || filtered[0].ID != "b" {
		t.Fatalf("latest=%+v %v", filtered, err)
	}
	filtered, err = r.ListCloudOperationsFiltered(ctx, domain.CloudOperationFilter{EnvironmentID: "env", InstanceID: "unrelated"})
	if err != nil || len(filtered) != 0 {
		t.Fatalf("filters not combined: %+v %v", filtered, err)
	}
}
