package main

import (
	"fmt"
	"path/filepath"
	"reflect"
)

// Only run identity, scheduling output and scoped quota authorization may differ
// in this follow-up. Derived preparation and allocation fields remain compared.
func matchedScientificInput(submitted obj) (obj, error) {
	input := clone(submitted)
	run, _ := input["Run"].(map[string]any)
	if run["mode"] != "real" || run["seed"] != float64(1) {
		return nil, fmt.Errorf("matched observation requires real mode and seed 1")
	}
	delete(input, "Run")
	delete(input, "Plan")
	resources, _ := input["Resources"].([]any)
	for _, raw := range resources {
		resource, _ := raw.(map[string]any)
		metadata, _ := resource["metadata"].(map[string]any)
		delete(metadata, "workspaceStorageQuotaOverride")
	}
	return input, nil
}

func exportMatchedMedium(base, root, prefix string) error {
	if err := auditMatrix(base, root); err != nil {
		return err
	}
	var original []obj
	read(filepath.Join(root, "matrix-results.json"), &original)
	dir := filepath.Join(root, "prism-time-followup")
	var request, plan, submitted obj
	read(filepath.Join(dir, "medium-request.json"), &request)
	read(filepath.Join(dir, "medium-plan.json"), &plan)
	runID := prefix + "-medium-real"
	read(filepath.Join(dir, runID+"-submitted.json"), &submitted)
	baseline, err := matchedScientificInput(submitted)
	if err != nil {
		return err
	}
	records := []obj{}
	for _, record := range original {
		if record["environment"] != "medium" {
			continue
		}
		sourceDir := fmt.Sprint(record["algorithm"])
		if sourceDir == "moheft" {
			sourceDir += "-" + fmt.Sprint(record["objective"])
		}
		var peer obj
		read(filepath.Join(root, sourceDir, fmt.Sprint(record["realRunId"])+"-submitted.json"), &peer)
		input, err := matchedScientificInput(peer)
		if err != nil || !reflect.DeepEqual(baseline, input) {
			return fmt.Errorf("scientific inputs differ for %v: %v", record["realRunId"], err)
		}
		records = append(records, record)
	}
	if len(records) != 3 {
		return fmt.Errorf("expected three verified medium peers")
	}
	client := apiClient{base}
	var monitor, simulation, authoritativePlan obj
	if err := client.call("GET", "/execution-runs/"+runID+"/monitor/", nil, &monitor); err != nil {
		return err
	}
	real := monitor["run"].(map[string]any)
	if real["status"] != "completed" || real["completedActivityCount"] != float64(58) || real["mode"] != "real" || real["seed"] != float64(1) || real["schedulePlanId"] != plan["id"] {
		return fmt.Errorf("matched PRISM real observation is incomplete or uses another plan")
	}
	if err := client.call("GET", "/schedule-plans/"+fmt.Sprint(plan["id"])+"/", nil, &authoritativePlan); err != nil {
		return err
	}
	signature, err := physicalSignature(request, plan)
	if err != nil {
		return err
	}
	for _, observed := range []obj{authoritativePlan, submitted["Plan"].(map[string]any)} {
		actual, err := physicalSignature(submitted, observed)
		if err != nil || actual != signature || observed["algorithm"] != "prism-time" || observed["algorithmVersion"] != plan["algorithmVersion"] || !reflect.DeepEqual(observed["predicted"], plan["predicted"]) {
			return fmt.Errorf("matched PRISM selected/submitted/authoritative plan differs")
		}
	}
	if err := auditPhysicalHandles(monitor, plan); err != nil {
		return err
	}
	if err := client.call("GET", "/execution-runs/"+prefix+"-medium-simulation/?view=status", nil, &simulation); err != nil {
		return err
	}
	sim := simulation["run"].(map[string]any)
	if sim["status"] != "completed" || sim["completedActivityCount"] != float64(58) || sim["mode"] != "simulation" || sim["seed"] != float64(1) || sim["schedulePlanId"] != plan["id"] {
		return fmt.Errorf("matched PRISM simulation differs")
	}
	billing := monitor["cloudBilling"].(map[string]any)
	if billing["status"] != "estimated" {
		return fmt.Errorf("incomplete lifecycle cost estimate")
	}
	for _, raw := range billing["instances"].([]any) {
		if raw.(map[string]any)["status"] != "destroyed" {
			return fmt.Errorf("matched PRISM cloud teardown incomplete")
		}
	}
	metadata := plan["metadata"].(map[string]any)
	if metadata["costModel"] != "resource-active-window" {
		return fmt.Errorf("unrecognized native PRISM cost basis")
	}
	records = append(records, obj{"algorithm": "prism-time", "objective": "time", "environment": "medium", "plan": plan, "simulation": sim, "real": real, "realRunId": runID, "physicalSignature": signature, "sharedObservation": false, "cloudBilling": billing, "plannedCostBasis": "resource-active-window"})
	save(filepath.Join(dir, "medium-final-monitor.json"), monitor)
	save(filepath.Join(dir, "matrix-results.json"), records)
	if err := exportMatrix(dir, records); err != nil {
		return err
	}
	allRecords := append(clone(original), records[len(records)-1])
	if err := exportExecutionCSV(filepath.Join(root, "all-execution-results.csv"), allRecords); err != nil {
		return err
	}
	save(filepath.Join(dir, "matched-engine-audit.json"), obj{"engineAuditPassed": true, "logicalEntries": 4, "sameScientificExecutionInputs": true, "scope": "completed daemon plans/runs/handles, seed, submitted scientific inputs and complete associated VM/disk lifecycle estimates; independent provider and remote-storage cleanup audits remain required"})
	return nil
}
