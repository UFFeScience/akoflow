package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestOfflineReplanRejectsRealWithoutCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input.json"), filepath.Join(dir, "case")
	request := ports.ExecutionRequest{Run: domain.ExecutionRun{Mode: domain.ExecutionModeReal}}
	payload, _ := json.Marshal(request)
	if err := os.WriteFile(input, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := exportCase(input, output, true); err == nil {
		t.Fatal("real mode accepted")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("real request created output")
	}
}

func TestOfflineReplanExportsNewPlanAndExactSimulationInput(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input.json"), filepath.Join(dir, "case")
	request := ports.ExecutionRequest{
		Run:              domain.ExecutionRun{ID: "test", Mode: domain.ExecutionModeSimulation},
		Workflow:         domain.WorkflowVersion{ID: "workflow", Activities: []domain.Activity{{ID: "a", ActivityTypeID: "type", Kind: domain.ActivityKindTask, Resources: domain.ActivityResources{CPU: 1, MemoryBytes: 1}, Simulation: &domain.ActivitySimulation{DurationSeconds: 1}}}},
		ExecutionScope:   domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}},
		Resources:        []domain.Resource{{ID: "r", EnvironmentVersionID: "environment", ExecutionTarget: domain.ExecutionTargetDirect, CPUCores: 1, CPUCapacity: 1, MemoryBytes: 1024, ComputeSpeedup: 1, Schedulable: true}},
		ActivityProfiles: []domain.ActivityResourceProfile{{ID: "profile", ActivityTypeID: "type", ResourceID: "r", RuntimeSeconds: 1, ModelVersion: "reference-work-v1"}},
	}
	payload, _ := json.Marshal(request)
	if err := os.WriteFile(input, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := exportCase(input, output, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"request.json", "candidates.json", "simulation.json", "platform.xml"} {
		info, err := os.Stat(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatal("private export has unsafe permissions")
		}
	}
	encoded, _ := os.ReadFile(filepath.Join(output, "request.json"))
	var selected ports.ExecutionRequest
	if err := json.Unmarshal(encoded, &selected); err != nil {
		t.Fatal(err)
	}
	if selected.Plan.Algorithm != "prism-time" || len(selected.Plan.Assignments) != 1 || selected.Run.SchedulePlanID != selected.Plan.ID {
		t.Fatal("new plan not exported")
	}
}
