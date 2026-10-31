package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatrixCleanupOnlyDestroysStoppedOwnedWorkersDuringRun(t *testing.T) {
	destroyed := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/execution-runs/run/monitor/" {
			entries := []obj{}
			for _, id := range []string{"stopped", "ready", "foreign", "destroyed"} {
				owner := "run"
				if id == "foreign" {
					owner = "other"
				}
				entries = append(entries, obj{"run": obj{"instanceId": id, "executionRunId": owner}})
			}
			json.NewEncoder(w).Encode(obj{"infrastructureRuns": entries})
			return
		}
		if r.Method == "POST" {
			destroyed = append(destroyed, r.URL.Path)
			w.Write([]byte(`{}`))
			return
		}
		switch r.URL.Path {
		case "/cloud-instances/stopped/":
			json.NewEncoder(w).Encode(obj{"status": "stopped"})
		case "/cloud-instances/ready/":
			json.NewEncoder(w).Encode(obj{"status": "ready"})
		case "/cloud-instances/destroyed/":
			json.NewEncoder(w).Encode(obj{"status": "destroyed"})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if err := cleanupMatrixCloud(apiClient{server.URL}, "run", false); err != nil {
		t.Fatal(err)
	}
	if len(destroyed) != 1 || destroyed[0] != "/cloud-instances/stopped/destroy/" {
		t.Fatalf("destroyed=%v", destroyed)
	}
}
