package workflow_engine_api_handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	resourcerepo "github.com/UFFeScience/akoflow/internal/infrastructure/database/resource"
)

func TestInventoryOnlyResourceDocumentationRequest(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO environments(id, name) VALUES ('simulation-example', 'Simulation example')`,
		`INSERT INTO environment_versions(id, environment_id, version, status, network_model, interference_model, cost_model, configuration_hash) VALUES ('simulation-example-v1', 'simulation-example', 1, 'published', '{}', '{}', '{}', 'test')`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	store := resourcerepo.New(db)
	handler := &Handler{resources: store}
	const body = `{
		"id": "inventory-only-node",
		"environmentVersionId": "simulation-example-v1",
		"type": "fog_device",
		"name": "Inventory-only node",
		"providerId": "inventory-only-node",
		"cpuCores": 2,
		"cpuCapacity": 2,
		"memoryBytes": 2147483648,
		"computeSpeedup": 1,
		"schedulable": false
	}`
	request := httptest.NewRequest(http.MethodPost, "/resources/", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.CreateResource(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create inventory resource: %d %s", response.Code, response.Body.String())
	}
	stored, err := store.FindByID(ctx, "inventory-only-node")
	if err != nil || stored == nil || stored.EnvironmentVersionID != "simulation-example-v1" || stored.CPUCores != 2 || stored.ComputeSpeedup != 1 || stored.Schedulable {
		t.Fatalf("stored resource: %#v, %v", stored, err)
	}
	var bindings int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_runtime_bindings WHERE resource_id='inventory-only-node'`).Scan(&bindings); err != nil || bindings != 0 {
		t.Fatalf("resource upsert unexpectedly created a runtime binding: count=%d error=%v", bindings, err)
	}
}
