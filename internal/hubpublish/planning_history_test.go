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
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		var input struct {
			Activities map[string]ActivityHistoryProfile
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if profile := input.Activities["portable-id"]; profile.Runtime != "Slurm" || profile.Architecture != "amd64" || profile.AllocatedCPU != 1 {
			t.Errorf("unexpected profile: %+v", profile)
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
		"imported-version": {URI: definitionURI, HubOrigin: manager.state.HubURL, LocalHash: digest(data)},
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
	workflow.Version.Activities[0].Command.Entrypoint = "false"
	if profiles := manager.HistoricalProfiles(context.Background(), workflow, resources, environments); len(profiles) != 0 || requests != 1 {
		t.Fatalf("edited workflow used history: %+v with %d requests", profiles, requests)
	}
}
