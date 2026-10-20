package data

import (
	"context"
	"fmt"
	"testing"
)

func TestGeneratedDataPageCountsLocationsWithoutDuplicatingObjects(t *testing.T) {
	r := testRepository(t)
	ctx := context.Background()
	if _, err := r.db.Exec(`INSERT INTO data_objects(id,workflow_version_id,logical_name) VALUES ('data','workflow-version','result')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := r.db.Exec(`INSERT INTO data_object_instances(id,data_object_id,execution_run_id,producer_activity_id,relative_path,size_bytes) VALUES (?,'data','run','activity',?,42)`, fmt.Sprintf("i%d", i), fmt.Sprintf("file%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"l1", "l2"} {
		if _, err := r.db.Exec(`INSERT INTO data_locations(id,data_object_instance_id,resource_id,execution_run_id,uri,status) VALUES (?,'i1','resource','run',?,'available')`, id, "file:///"+id); err != nil {
			t.Fatal(err)
		}
	}
	page, err := r.ListGeneratedDataPage(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || !page.HasNext || len(page.Items) != 1 || page.Items[0].ID != "i1" || page.Items[0].Locations != 2 || page.Items[0].LogicalName != "result" || page.Items[0].RunID != "run" {
		t.Fatalf("page=%+v", page)
	}
	empty, err := r.ListGeneratedDataPage(ctx, 4, 1)
	if err != nil || len(empty.Items) != 0 || empty.HasNext || empty.Total != 3 {
		t.Fatalf("empty=%+v, %v", empty, err)
	}
}
