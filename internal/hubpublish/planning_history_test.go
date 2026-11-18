package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestPlanningHistoryAppliesOnlyToUneditedImportAndCompatibleResource(t *testing.T) {
	requests := 0
	selected := &ExecutionReference{ExperimentID: "tested-experiment", RunID: "chosen-run"}
	useSelected := false
	replyFromOldHub := false
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		var input struct {
			Activities map[string]ActivityHistoryProfile
			Reference  *ExecutionReference
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if profile := input.Activities["portable-id"]; profile.Runtime != "Slurm" || profile.Architecture != "amd64" || profile.AllocatedCPU != 1 {
			t.Errorf("unexpected profile: %+v", profile)
		}
		if useSelected {
			if input.Reference == nil || *input.Reference != *selected {
				t.Errorf("missing selected reference: %+v", input.Reference)
			}
			if !replyFromOldHub {
				_, _ = w.Write([]byte(`{"portable-id":{"medianSeconds":12,"minSeconds":12,"maxSeconds":12,"sampleCount":1,"selectedReference":true}}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"portable-id":{"medianSeconds":8,"minSeconds":6,"maxSeconds":9,"sampleCount":3}}`))
	})
	workflow := domain.WorkflowDefinition{Name: "imported", Version: domain.WorkflowVersion{ID: "imported-version", Activities: []domain.Activity{
		{ID: "local-id", ExternalID: "portable-id", Name: "task", Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
			Command: domain.ActivityCommand{Entrypoint: "true"}, Resources: domain.ActivityResources{CPU: 1}},
	}}}
	data, err := apirequests.FromDomain(workflow).YAML()
	if err != nil {
		t.Fatal(err)
	}
	manager.state.WorkflowSources = map[string]WorkflowSource{
		"imported-version": {URI: "akoflow::test/workflow:v1", PublicationID: "35746b14-7dc2-454c-a74f-902d58b50c94", Version: 1, HubOrigin: manager.state.HubURL, LocalHash: digest(data)},
	}
	resources := []domain.Resource{{ID: "slurm", Architecture: "x86_64"}, {ID: "unbound", Architecture: "amd64"}}
	environments := []domain.EnvironmentDefinition{{
		Runtimes:        []domain.EnvironmentRuntime{{ID: "hpc", Driver: domain.RuntimeDriverSlurm, Mode: domain.RuntimeModeExecution}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "slurm", RuntimeID: "hpc", Enabled: true}},
	}}
	profiles := manager.HistoricalProfiles(context.Background(), workflow, resources, environments)
	if len(profiles) != 1 || profiles[0].ActivityID != "local-id" || profiles[0].ResourceID != "slurm" || profiles[0].RuntimeSeconds != 8 {
		t.Fatalf("unexpected historical profiles: %+v", profiles)
	}
	source := manager.state.WorkflowSources["imported-version"]
	source.Reference = selected
	manager.state.WorkflowSources["imported-version"] = source
	useSelected = true
	profiles = manager.HistoricalProfiles(context.Background(), workflow, resources, environments)
	if len(profiles) != 1 || profiles[0].RuntimeSeconds != 12 || profiles[0].SampleSize != 1 {
		t.Fatalf("planning ignored the selected run: %+v", profiles)
	}
	replyFromOldHub = true
	if profiles := manager.HistoricalProfiles(context.Background(), workflow, resources, environments); len(profiles) != 0 {
		t.Fatalf("planning used unselected history from an old Hub: %+v", profiles)
	}
	workflow.Version.Activities[0].Command.Entrypoint = "false"
	if profiles := manager.HistoricalProfiles(context.Background(), workflow, resources, environments); len(profiles) != 0 || requests != 3 {
		t.Fatalf("edited workflow used history: %+v with %d requests", profiles, requests)
	}
}
