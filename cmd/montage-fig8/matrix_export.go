package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Export raw seconds and explicit cost bases; forecast cost functions are not
// interchangeable with provider-lifecycle estimates or invoices.
func exportMatrix(root string, records []obj) error {
	file, err := os.Create(filepath.Join(root, "matrix-results.csv"))
	if err != nil {
		return err
	}
	defer file.Close()
	w := csv.NewWriter(file)
	header := []string{"environment", "algorithm", "objective", "plan_id", "simulation_run_id", "real_run_id", "shared_real_observation", "physical_signature", "planned_makespan_seconds", "simulated_makespan_seconds", "real_makespan_seconds", "planned_cost_usd", "planned_cost_basis", "simulation_cost_usd", "real_run_cost_usd", "cloud_lifecycle_estimated_cost_usd", "cloud_lifecycle_cost_status", "completed_activities"}
	if err = w.Write(header); err != nil {
		return err
	}
	num := func(v any) string {
		if f, ok := v.(float64); ok {
			return strconv.FormatFloat(f, 'g', -1, 64)
		}
		return ""
	}
	for _, record := range records {
		plan := record["plan"].(map[string]any)
		prediction := plan["predicted"].(map[string]any)
		simulation := record["simulation"].(map[string]any)
		real := record["real"].(map[string]any)
		billing, _ := record["cloudBilling"].(map[string]any)
		basis := "vm-forecast"
		if record["algorithm"] == "moheft" {
			basis = "original-task-additive"
		}
		row := []string{fmt.Sprint(record["environment"]), fmt.Sprint(record["algorithm"]), fmt.Sprint(record["objective"]), fmt.Sprint(plan["id"]), fmt.Sprint(simulation["id"]), fmt.Sprint(record["realRunId"]), fmt.Sprint(record["sharedObservation"]), fmt.Sprint(record["physicalSignature"]), num(prediction["makespanSeconds"]), num(simulation["makespanSeconds"]), num(real["makespanSeconds"]), num(prediction["cost"]), basis, num(simulation["cost"]), num(real["cost"]), num(billing["estimatedCost"]), fmt.Sprint(billing["status"]), num(real["completedActivityCount"])}
		if err = w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
