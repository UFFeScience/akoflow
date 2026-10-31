package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestExportPreservesTransferUnitsAndMissingValues(t *testing.T) {
	dir := t.TempDir()
	record := obj{
		"plannedCostBasis": "resource-active-window",
		"plan":             obj{"id": "plan", "predicted": obj{}},
		"simulation":       obj{"transferredBytes": 100., "breakdown": obj{"transferSeconds": 4.5}},
		"real": obj{
			"startedAt": "2026-10-31T00:00:00Z", "finishedAt": "2026-10-31T00:00:10Z",
			"transferredBytes": 0., "breakdown": obj{"transferSeconds": 12.5},
		},
	}
	if err := exportMatrix(dir, []obj{record}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(dir, "matrix-results.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	for i, name := range rows[0] {
		fields[name] = rows[1][i]
	}
	for field, expected := range map[string]string{
		"planned_cost_basis":           "resource-active-window",
		"simulation_transferred_bytes": "100", "real_transferred_bytes": "0",
		"simulation_transfer_work_seconds": "4.5", "real_transfer_work_seconds": "12.5",
		"real_wall_seconds": "10", "real_makespan_seconds": "",
	} {
		if fields[field] != expected {
			t.Fatalf("%s = %q, expected %q", field, fields[field], expected)
		}
	}
}
