package main

import (
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestFigure8ScenariosUseExactCapacityAndClosedReferences(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, e := os.Stat(filepath.Join(root, "storage/montage-runtime-stage-calibration-20261011/medium-independent-prepared-v1/request.json")); e != nil {
		t.Skip("archived calibration fixture unavailable")
	}
	cwd, _ := os.Getwd()
	if e := os.Chdir(root); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(cwd)
	var source ports.ExecutionRequest
	read("storage/montage-runtime-stage-calibration-20261011/medium-independent-prepared-v1/request.json", &source)
	var hpc, cloud domain.EnvironmentDefinition
	read("storage/montage-independent-20261011/hpc-clean-catalog.json", &hpc)
	read("storage/montage-independent-20261011/gcp-clean-catalog.json", &cloud)
	var targets []obj
	read("storage/montage-independent-20261011/targets-preflight.json", &targets)
	for _, tc := range []struct {
		kind                    string
		slots, workers, targets int
	}{{"hpc", 64, 1, 0}, {"micro", 11, 11, 10}, {"medium", 5, 3, 2}} {
		t.Run(tc.kind, func(t *testing.T) {
			env, req, ts := build(source, hpc, cloud, targets, tc.kind, "203.0.113.4")
			slots, workers := 0., 0
			ids := map[string]bool{}
			for _, r := range env.Resources {
				ids[r.ID] = true
				if r.Schedulable {
					slots += r.CPUCapacity
					workers++
				}
			}
			if slots != float64(tc.slots) || workers != tc.workers || len(ts) != tc.targets {
				t.Fatalf("capacity got %v/%d/%d", slots, workers, len(ts))
			}
			for _, l := range req.NetworkTopology.Links {
				if !ids[l.SourceResourceID] || !ids[l.TargetResourceID] {
					t.Fatal("link escaped scope", l.ID)
				}
			}
			for _, p := range env.Profiles {
				if !ids[p.ResourceID] {
					t.Fatal("profile escaped scope")
				}
			}
			for _, c := range env.Connections {
				if c.Type == "ssh" && c.Configuration["eligibleNodeFallback"] != false {
					t.Fatal("fallback changes physical node")
				}
			}
			if len(req.Workflow.Activities) != 58 || req.Workflow.ID != source.Workflow.ID {
				t.Fatal("workflow changed")
			}
		})
	}
}
func TestRemapIdentifiersDoesNotRewriteCommands(t *testing.T) {
	v := obj{"old": []any{"old", "echo old"}}
	got := remap(v, map[string]string{"old": "new"}).(map[string]any)
	a := got["new"].([]any)
	if a[0] != "new" || a[1] != "echo old" {
		t.Fatal(got)
	}
}
