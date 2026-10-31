package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type apiClient struct{ base string }

func (c apiClient) call(method, path string, v any, out any) error {
	var b []byte
	var e error
	if v != nil {
		b, e = json.Marshal(v)
		if e != nil {
			return e
		}
	}
	req, e := http.NewRequest(method, c.base+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("AKOFLOW_API_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, e := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	b, e = io.ReadAll(res.Body)
	if e != nil {
		return e
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s", method, path, b)
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}
func (c apiClient) exists(path string) bool {
	req, e := http.NewRequest("GET", c.base+path, nil)
	if e != nil {
		return false
	}
	if token := os.Getenv("AKOFLOW_API_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, e := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if e != nil {
		return false
	}
	defer r.Body.Close()
	return r.StatusCode == 200
}
func (c apiClient) wait(path, field string, limit time.Duration) (obj, error) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		var result obj
		if e := c.call("GET", path, nil, &result); e != nil {
			return nil, e
		}
		record, _ := result[field].(map[string]any)
		status, _ := record["status"].(string)
		switch status {
		case "completed":
			return result, nil
		case "failed", "cancelled":
			return result, fmt.Errorf("%s: %v", path, result)
		}
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("deadline exceeded: %s; execution may still be running", path)
}
func campaign(action, base, dir, kind, mode string, attempt int, campaignID string, acceptQuotaEstimate bool) error {
	if kind != "hpc" && kind != "micro" && kind != "medium" {
		return fmt.Errorf("case must be hpc, micro or medium")
	}
	client := apiClient{base}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(campaignID) {
		return fmt.Errorf("invalid campaign identifier")
	}
	session := campaignID + "-" + kind
	var request obj
	read(filepath.Join(dir, kind+"-request.json"), &request)
	if action == "plan" {
		if !client.exists("/planning-sessions/" + session + "/") {
			if e := client.call("POST", "/planning-sessions/", obj{"id": session, "workflowVersionId": "montage-independent-work-v1-v1", "executionScopeId": "fig8-" + kind + "-scope", "networkTopologyId": "fig8-" + kind + "-network", "algorithms": []obj{{"id": "heft"}}}, nil); e != nil {
				return e
			}
		}
		result, e := client.wait("/planning-sessions/"+session+"/", "session", 20*time.Minute)
		save(filepath.Join(dir, kind+"-session.json"), result)
		if e != nil {
			return e
		}
		var candidates []obj
		if e = client.call("GET", "/planning-sessions/"+session+"/candidates/", nil, &candidates); e != nil {
			return e
		}
		save(filepath.Join(dir, kind+"-candidates.json"), candidates)
		var chosen obj
		for _, candidate := range candidates {
			if candidate["feasible"] == true {
				if chosen != nil {
					return fmt.Errorf("HEFT returned multiple feasible candidates")
				}
				chosen = candidate
			}
		}
		if chosen == nil {
			return fmt.Errorf("HEFT returned no feasible candidate; inspect %s", kind+"-candidates.json")
		}
		var plan obj
		if e = client.call("POST", "/planning-sessions/"+session+"/candidates/"+chosen["id"].(string)+"/select/", obj{}, &plan); e != nil {
			return e
		}
		save(filepath.Join(dir, kind+"-plan.json"), plan)
		fmt.Printf("%s HEFT frozen: %v\n", kind, plan["predicted"])
		return nil
	}
	if action != "run" || mode != "simulation" && mode != "real" {
		return fmt.Errorf("action must be prepare, plan or run; run requires --mode simulation or real")
	}
	var plan obj
	read(filepath.Join(dir, kind+"-plan.json"), &plan)
	request["Plan"] = plan
	runID := session + "-" + mode
	if attempt < 1 {
		return fmt.Errorf("attempt must be positive")
	}
	if attempt > 1 {
		runID = fmt.Sprintf("%s-attempt%d", runID, attempt)
	}
	request["Run"] = obj{"id": runID, "mode": mode, "seed": 1}
	if acceptQuotaEstimate {
		for _, raw := range request["Resources"].([]any) {
			r := raw.(map[string]any)
			if r["type"] != "hpc_machine" {
				continue
			}
			metadata := r["metadata"].(map[string]any)
			metadata["workspaceStorageQuotaOverride"] = obj{"runId": runID, "acceptedRisk": true, "source": "explicit-user-authorization", "reason": "User requested standard home storage with consumer-barrier cleanup after prior successful runs; retain live quota observation and operating-system enforcement."}
		}
	}
	snapshot := filepath.Join(dir, runID+"-submitted.json")
	if !client.exists("/execution-runs/" + runID + "/?view=status") {
		if mode == "real" {
			var runs []obj
			if e := client.call("GET", "/execution-runs/", nil, &runs); e != nil {
				return e
			}
			matchedSimulation := false
			for _, r := range runs {
				if nested, ok := r["run"].(map[string]any); ok {
					r = nested
				}
				if r["mode"] == "simulation" && r["status"] == "completed" && r["schedulePlanId"] == plan["id"] {
					matchedSimulation = true
				}
				if r["mode"] == "real" && (r["status"] == "created" || r["status"] == "running" || r["status"] == "preparing") {
					return fmt.Errorf("real execution %v still active; Figure 8 shares Diablo03 and must run sequentially", r["id"])
				}
			}
			if !matchedSimulation {
				return fmt.Errorf("complete a simulation of this exact plan before real execution")
			}
		}
		if _, e := os.Stat(snapshot); e == nil {
			return fmt.Errorf("snapshot exists but run missing; inspect before resubmitting")
		}
		save(snapshot, request)
		if e := client.call("POST", "/execution-runs/", request, nil); e != nil {
			return e
		}
	}
	fmt.Println("Monitoring", runID)
	result, e := client.wait("/execution-runs/"+runID+"/?view=status", "run", 8*time.Hour)
	save(filepath.Join(dir, runID+"-result.json"), result)
	if e == nil {
		fmt.Printf("Completed %s: %v\n", runID, result["run"])
	}
	return e
}
