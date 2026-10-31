package main

import (
	"fmt"
	"path/filepath"
	"time"
)

// matrix runs unique physical plans sequentially; it never invents independent
// observations for duplicate plans. Every record keeps its real source run.
func matrix(base, root string, acceptQuota bool) error {
	client := apiClient{base}
	records := []obj{}
	observations := map[string]string{}
	observationDirectories := map[string]string{}
	for _, spec := range []struct{ dir, algorithm, objective, prefix string }{
		{"heft", "heft", "time", "fig8-heft-canonical-20261031-v2"},
		{"moheft-time", "moheft", "time", "fig8-moheft-time-canonical-20261031-v2"},
		{"moheft-cost", "moheft", "cost", "fig8-moheft-cost-canonical-20261031-v2"},
	} {
		dir := filepath.Join(root, spec.dir)
		for _, kind := range []string{"hpc", "micro", "medium"} {
			var request, plan, simulation obj
			read(filepath.Join(dir, kind+"-request.json"), &request)
			read(filepath.Join(dir, kind+"-plan.json"), &plan)
			simulationID := spec.prefix + "-" + kind + "-simulation"
			if err := client.call("GET", "/execution-runs/"+simulationID+"/?view=status", nil, &simulation); err != nil {
				return err
			}
			sim := simulation["run"].(map[string]any)
			if sim["status"] != "completed" || sim["completedActivityCount"] != float64(58) || sim["schedulePlanId"] != plan["id"] {
				return fmt.Errorf("invalid simulation %s: %v", simulationID, sim)
			}
			signature, err := physicalSignature(request, plan)
			if err != nil {
				return err
			}
			runID, reused := observations[signature]
			if !reused {
				runID = spec.prefix + "-" + kind + "-real"
				done := make(chan error, 1)
				go func() {
					done <- campaign("run", base, dir, kind, "real", 1, spec.prefix, acceptQuota, spec.algorithm, spec.objective)
				}()
				ticker := time.NewTicker(15 * time.Second)
				running := true
				for running {
					select {
					case err = <-done:
						running = false
					case <-ticker.C:
						// Only instances that the consumer barrier has already stopped may
						// be destroyed while the workflow is active. Ready instances stay put.
						if cleanupErr := cleanupMatrixCloud(client, runID, false); cleanupErr != nil {
							fmt.Println("Cleanup pending:", cleanupErr)
						}
					}
				}
				ticker.Stop()
				if err != nil {
					return err
				}
			}
			var result, monitor obj
			if err := client.call("GET", "/execution-runs/"+runID+"/?view=status", nil, &result); err != nil {
				return err
			}
			real := result["run"].(map[string]any)
			if real["status"] != "completed" || real["completedActivityCount"] != float64(58) {
				return fmt.Errorf("real run incomplete: %v", real)
			}
			// Verify the submitted snapshot, not just the intended local request.
			sourceDir := dir
			if reused {
				sourceDir = observationDirectories[signature]
			}
			var submitted obj
			read(filepath.Join(sourceDir, runID+"-submitted.json"), &submitted)
			observedSignature, err := physicalSignature(submitted, submitted["Plan"].(map[string]any))
			if err != nil || observedSignature != signature {
				return fmt.Errorf("physical signature mismatch for %s", runID)
			}
			deadline := time.Now().Add(20 * time.Minute)
			for {
				err = cleanupMatrixCloud(client, runID, true)
				if err == nil {
					break
				}
				if time.Now().After(deadline) {
					return err
				}
				fmt.Println("Waiting for cloud teardown:", err)
				time.Sleep(10 * time.Second)
			}
			if err := client.call("GET", "/execution-runs/"+runID+"/monitor/", nil, &monitor); err != nil {
				return err
			}
			save(filepath.Join(dir, kind+"-real-monitor.json"), monitor)
			record := obj{"algorithm": spec.algorithm, "objective": spec.objective, "environment": kind, "plan": plan, "simulation": sim, "real": real, "realRunId": runID, "physicalSignature": signature, "sharedObservation": reused, "cloudBilling": monitor["cloudBilling"]}
			records = append(records, record)
			observations[signature] = runID
			observationDirectories[signature] = sourceDir
			save(filepath.Join(root, "matrix-results.json"), records)
			fmt.Printf("Verified %s/%s: %s shared=%v\n", spec.dir, kind, runID, reused)
		}
	}
	return nil
}

func cleanupMatrixCloud(client apiClient, runID string, terminal bool) error {
	var monitor obj
	if err := client.call("GET", "/execution-runs/"+runID+"/monitor/", nil, &monitor); err != nil {
		return err
	}
	seen := map[string]bool{}
	pending := false
	entries, _ := monitor["infrastructureRuns"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		operation, _ := entry["run"].(map[string]any)
		id, _ := operation["instanceId"].(string)
		if id == "" || seen[id] || operation["executionRunId"] != runID {
			continue
		}
		seen[id] = true
		var instance obj
		if err := client.call("GET", "/cloud-instances/"+id+"/", nil, &instance); err != nil {
			return err
		}
		status := instance["status"]
		if status == "destroyed" {
			continue
		}
		pending = true
		if status == "stopped" || terminal && status == "ready" {
			if err := client.call("POST", "/cloud-instances/"+id+"/destroy/", obj{}, nil); err != nil {
				return err
			}
			fmt.Println("Destroying run-owned instance", id)
		}
	}
	if pending && terminal {
		return fmt.Errorf("run %s still has cloud infrastructure", runID)
	}
	return nil
}
