package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// auditMatrix re-reads authoritative runs and plans before publication. Passing
// this engine audit does not replace the independent provider/remote-disk audit.
func auditMatrix(base, root string) error {
	var records []obj
	read(filepath.Join(root, "matrix-results.json"), &records)
	client := apiClient{base}
	seen := map[string]bool{}
	seenReal := map[string]string{}
	verified := []obj{}
	for _, record := range records {
		algorithm, objective, kind := fmt.Sprint(record["algorithm"]), fmt.Sprint(record["objective"]), fmt.Sprint(record["environment"])
		if (algorithm != "heft" && algorithm != "moheft") || (objective != "time" && objective != "cost") || (algorithm == "heft" && objective != "time") || (kind != "hpc" && kind != "micro" && kind != "medium") {
			return fmt.Errorf("unexpected matrix entry")
		}
		key := algorithm + "/" + objective + "/" + kind
		if seen[key] {
			return fmt.Errorf("duplicate matrix entry %s", key)
		}
		seen[key] = true
		dir := algorithm
		if algorithm == "moheft" {
			dir += "-" + objective
		}
		var request obj
		read(filepath.Join(root, dir, kind+"-request.json"), &request)
		plan := record["plan"].(map[string]any)
		if plan["algorithm"] != algorithm {
			return fmt.Errorf("logical algorithm mismatch for %s", key)
		}
		signature, err := physicalSignature(request, plan)
		if err != nil {
			return err
		}
		if signature != record["physicalSignature"] {
			return fmt.Errorf("stored signature changed for %s", key)
		}
		var logicalPlan obj
		if err = client.call("GET", "/schedule-plans/"+fmt.Sprint(plan["id"])+"/", nil, &logicalPlan); err != nil {
			return err
		}
		logicalSignature, err := physicalSignature(request, logicalPlan)
		if err != nil {
			return err
		}
		if logicalSignature != signature || logicalPlan["algorithm"] != algorithm || logicalPlan["algorithmVersion"] != plan["algorithmVersion"] || !reflect.DeepEqual(logicalPlan["predicted"], plan["predicted"]) {
			return fmt.Errorf("frozen logical plan or prediction differs from daemon for %s", key)
		}
		var simulation obj
		simulationID := record["simulation"].(map[string]any)["id"].(string)
		if err = client.call("GET", "/execution-runs/"+simulationID+"/?view=status", nil, &simulation); err != nil {
			return err
		}
		sim := simulation["run"].(map[string]any)
		if sim["mode"] != "simulation" || sim["seed"] != float64(1) || sim["status"] != "completed" || sim["completedActivityCount"] != float64(58) || sim["schedulePlanId"] != plan["id"] {
			return fmt.Errorf("simulation mismatch for %s", key)
		}
		runID := record["realRunId"].(string)
		previousSignature, reused := seenReal[runID]
		if record["sharedObservation"] != reused || (reused && previousSignature != signature) {
			return fmt.Errorf("unproven or incorrectly labeled shared observation for %s", key)
		}
		seenReal[runID] = signature
		var monitor, observedPlan obj
		if err = client.call("GET", "/execution-runs/"+runID+"/monitor/", nil, &monitor); err != nil {
			return err
		}
		run := monitor["run"].(map[string]any)
		if run["mode"] != "real" || run["seed"] != float64(1) || run["status"] != "completed" || run["completedActivityCount"] != float64(58) {
			return fmt.Errorf("real run incomplete for %s", key)
		}
		if err = client.call("GET", "/schedule-plans/"+fmt.Sprint(run["schedulePlanId"])+"/", nil, &observedPlan); err != nil {
			return err
		}
		actualSignature, err := physicalSignature(request, observedPlan)
		if err != nil {
			return err
		}
		if actualSignature != signature {
			return fmt.Errorf("authoritative executed plan differs for %s", key)
		}
		if err = auditPhysicalHandles(monitor, observedPlan); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		billing := monitor["cloudBilling"].(map[string]any)
		if billing["status"] != "estimated" {
			return fmt.Errorf("incomplete cloud lifecycle estimate for %s", key)
		}
		for _, raw := range billing["instances"].([]any) {
			if raw.(map[string]any)["status"] != "destroyed" {
				return fmt.Errorf("cloud instance not destroyed for %s", key)
			}
		}
		verified = append(verified, obj{"entry": key, "runId": runID, "executedPlanId": run["schedulePlanId"], "physicalSignature": signature, "checks": []string{"simulation completed 58 with selected plan", "real completed 58", "authoritative executed plan signature matches", "58 successful unique physical handles at selected resources", "HPC strictly on Diablo03", "all associated cloud instances destroyed", "cloud lifecycle estimate complete"}})
	}
	complete := len(seen) == 9
	save(filepath.Join(root, "matrix-engine-audit.json"), obj{"auditedAt": time.Now().UTC(), "expectedLogicalEntries": 9, "verifiedEntries": verified, "engineAuditPassed": complete, "scope": "daemon plans, simulations, physical handles and associated cloud lifecycle; provider resources and remote temporary storage require independent audit"})
	if !complete {
		return fmt.Errorf("only %d of 9 logical entries verified; campaign incomplete", len(seen))
	}
	return nil
}

func auditPhysicalHandles(monitor, plan obj) error {
	expected := map[string]string{}
	for _, raw := range plan["assignments"].([]any) {
		a := raw.(map[string]any)
		id := fmt.Sprint(a["activityId"])
		if expected[id] != "" {
			return fmt.Errorf("duplicate assignment")
		}
		expected[id] = fmt.Sprint(a["resourceId"])
	}
	handles, _ := monitor["handles"].([]any)
	attempts, _ := monitor["activityAttempts"].([]any)
	if len(expected) != 58 || len(handles) != 58 || len(attempts) != 58 {
		return fmt.Errorf("expected 58 assignments, physical handles and single activity attempts")
	}
	seen := map[string]bool{}
	for _, raw := range handles {
		h := raw.(map[string]any)
		id := fmt.Sprint(h["activityId"])
		resource := fmt.Sprint(h["resourceId"])
		if seen[id] || expected[id] != resource || h["status"] != "completed" || h["exitCode"] != float64(0) {
			return fmt.Errorf("physical handle mismatch for %s", id)
		}
		seen[id] = true
		if strings.Contains(resource, "plafrim") {
			metadata, _ := h["metadata"].(map[string]any)
			if metadata["allocatedNode"] != "diablo03" || metadata["eligibleNodeFallback"] != false {
				return fmt.Errorf("unexpected HPC node or fallback for %s", id)
			}
		}
	}
	return nil
}
