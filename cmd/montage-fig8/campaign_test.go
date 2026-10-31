package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRealExecutionRequiresSimulationAndNoActivePhysicalPeer(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		simulation, active, allowed bool
	}{{"ready", true, false, true}, {"shared node busy", true, true, false}, {"simulation missing", false, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			save(filepath.Join(dir, "hpc-request.json"), obj{"Resources": []obj{{"type": "hpc_machine", "metadata": obj{}}}})
			save(filepath.Join(dir, "hpc-plan.json"), obj{"id": "frozen-plan"})
			posted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == "GET" && r.URL.Path == "/execution-runs/":
					runs := []obj{}
					if tc.simulation {
						runs = append(runs, obj{"run": obj{"id": "sim", "mode": "simulation", "status": "completed", "schedulePlanId": "frozen-plan"}})
					}
					if tc.active {
						runs = append(runs, obj{"run": obj{"id": "other-case", "mode": "real", "status": "running"}})
					}
					json.NewEncoder(w).Encode(runs)
				case r.Method == "POST" && r.URL.Path == "/execution-runs/":
					posted = true
					var req obj
					if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
						t.Error(e)
					}
					if req["Plan"].(map[string]any)["id"] != "frozen-plan" {
						t.Error("changed plan")
					}
					resource := req["Resources"].([]any)[0].(map[string]any)
					override := resource["metadata"].(map[string]any)["workspaceStorageQuotaOverride"].(map[string]any)
					if override["runId"] != "test-hpc-real" || override["source"] != "explicit-user-authorization" {
						t.Error(override)
					}
					w.WriteHeader(201)
					w.Write([]byte(`{}`))
				case r.Method == "GET" && posted:
					w.Write([]byte(`{"run":{"status":"completed"}}`))
				default:
					w.WriteHeader(404)
					w.Write([]byte(`{}`))
				}
			}))
			defer server.Close()
			e := campaign("run", server.URL, dir, "hpc", "real", 1, "test", true)
			if (e == nil) != tc.allowed || posted != tc.allowed {
				t.Fatalf("err=%v posted=%v allowed=%v", e, posted, tc.allowed)
			}
		})
	}
}
