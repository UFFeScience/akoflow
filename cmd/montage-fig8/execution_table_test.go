package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionTableKeepsHistoricalRecoverySeparate(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, rows [][]string) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		w := csv.NewWriter(f)
		w.WriteAll(rows)
		if err := w.Error(); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	write("all-execution-results.csv", [][]string{
		{"environment", "algorithm", "objective", "real_run_id", "completed_activities", "shared_real_observation", "planned_makespan_seconds", "simulated_makespan_seconds", "real_makespan_seconds"},
		{"hpc", "heft", "time", "shared", "58", "false", "70", "80", "100"},
		{"hpc", "moheft", "time", "shared", "58", "true", "70", "80", "100"},
	})
	write("history.csv", [][]string{
		{"scenario", "planned", "simulated", "real", "runId", "recovered"},
		{"HPC + medium", "70", "120", "100", "historical", "True"},
	})
	if err := exportExecutionTable(dir, filepath.Join(dir, "history.csv")); err != nil {
		t.Fatal(err)
	}
	rows, err := readExecutionCSV(filepath.Join(dir, "execution-table.csv"))
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	if rows[0]["simulation_absolute_error_percent"] != "20" || rows[0]["real_minus_simulated_seconds"] != "20" {
		t.Fatal("current error denominator changed", rows[0])
	}
	if rows[1]["shared_observation"] != "true" || rows[2]["cohort"] != "historical-separate-campaign" || rows[2]["recovered"] != "true" || rows[2]["real_minus_simulated_seconds"] != "-20" {
		t.Fatal("shared/recovered observation semantics lost", rows)
	}
	tex, err := os.ReadFile(filepath.Join(dir, "execution-table.tex"))
	if err != nil || !strings.Contains(string(tex), "Historical PRISM observations (separate campaign)") || !strings.Contains(string(tex), "$^{\\dagger}$") {
		t.Fatal("historical recovery not labeled", err)
	}
}
