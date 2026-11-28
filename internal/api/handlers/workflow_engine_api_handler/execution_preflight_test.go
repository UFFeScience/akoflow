package workflow_engine_api_handler

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainpreflight "github.com/UFFeScience/akoflow/internal/domain/preflight"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
)

type preflightCommandStub struct {
	ExecutionCommandStore
	reports map[string]domainpreflight.Report
}

func (s *preflightCommandStub) SavePreflight(_ context.Context, report domainpreflight.Report) error {
	if s.reports == nil {
		s.reports = map[string]domainpreflight.Report{}
	}
	s.reports[report.ID] = report
	return nil
}
func (s *preflightCommandStub) FindPreflight(_ context.Context, id string) (*domainpreflight.Report, error) {
	value, found := s.reports[id]
	if !found {
		return nil, nil
	}
	return &value, nil
}

func preflightRequest() ports.ExecutionRequest {
	return ports.ExecutionRequest{Run: domain.ExecutionRun{ID: "run", Mode: domain.ExecutionModeReal}, Plan: domain.SchedulePlan{ID: "plan"}, Workflow: domain.WorkflowVersion{ID: "workflow"}}
}
func TestRequiredInputsDistinguishExistingFilesFromPredecessorOutputs(t *testing.T) {
	h := &Handler{storage: &storageNavigatorStub{err: errors.New("file missing")}}
	request := preflightRequest()
	request.Workflow.Activities = []domain.Activity{{ID: "consumer", Metadata: map[string]any{"requiredInputs": []domainpreflight.Input{{StorageID: "storage", Path: "input.csv"}}}}}
	probe := h.preflightInputs(request)[0]
	status, message, _ := probe.Run(context.Background())
	if status != "blocked" || !strings.Contains(message, "input.csv") {
		t.Fatalf("missing input approved: %s %s", status, message)
	}
	request.Workflow.Activities[0].Metadata["requiredInputs"] = []domainpreflight.Input{{ProducerActivityID: "producer", Path: "output.csv"}}
	request.Workflow.Dependencies = []domain.ActivityDependency{{ActivityID: "consumer", DependsOnActivityID: "producer"}}
	status, _, _ = h.preflightInputs(request)[0].Run(context.Background())
	if status != "passed" {
		t.Fatal("future output required prematurely")
	}
}
func TestSimulationPreflightDoesNotProbeInfrastructure(t *testing.T) {
	h := &Handler{connectionTest: func(context.Context, domain.EnvironmentConnection) ports.ConnectionHealth {
		t.Fatal("simulation probed infrastructure")
		return ports.ConnectionHealth{}
	}}
	request := preflightRequest()
	request.Run.Mode = domain.ExecutionModeSimulation
	report := h.evaluateExecutionPreflight(context.Background(), &request)
	if !report.Ready || report.Warnings != 1 || len(report.Checks) != 3 {
		t.Fatalf("unexpected simulation report: %+v", report)
	}
}
func TestPreflightBlocksLaunchOnFreshInputFailure(t *testing.T) {
	request := preflightRequest()
	request.Workflow.Activities = []domain.Activity{{ID: "activity", Metadata: map[string]any{"requiredInputs": []domainpreflight.Input{{StorageID: "storage", Path: "missing.csv"}}}}}
	request.Plan.Assignments = []domain.PlanAssignment{{ActivityID: "activity", ResourceID: "resource"}}
	request.Resources = []domain.Resource{{ID: "resource", Type: domain.ResourceLocalMachine}}
	request.Runtimes = []domain.EnvironmentRuntime{{ID: "runtime", Driver: domain.RuntimeDriverLocal, Mode: domain.RuntimeModeExecution}}
	request.RuntimeBindings = []domain.ResourceRuntimeBinding{{ResourceID: "resource", RuntimeID: "runtime", Enabled: true}}
	h := &Handler{storage: &storageNavigatorStub{err: errors.New("not found")}, connectionTest: func(context.Context, domain.EnvironmentConnection) ports.ConnectionHealth {
		return ports.ConnectionHealth{Healthy: true}
	}}
	request.AcknowledgeWarnings = true
	if err := h.PrepareExecution(context.Background(), &request); err == nil || !strings.Contains(err.Error(), "missing.csv") {
		t.Fatalf("launch escaped preflight: %v", err)
	}
}

func TestLaunchRechecksMutableInputsAfterApprovedReport(t *testing.T) {
	request := preflightRequest()
	request.Workflow.Activities = []domain.Activity{{ID: "activity", Metadata: map[string]any{
		"requiredInputs": []domainpreflight.Input{{StorageID: "storage", Path: "input.csv"}},
	}}}
	request.Plan.Assignments = []domain.PlanAssignment{{ActivityID: "activity", ResourceID: "resource"}}
	request.Resources = []domain.Resource{{ID: "resource", Type: domain.ResourceLocalMachine}}
	request.Runtimes = []domain.EnvironmentRuntime{{ID: "runtime", Driver: domain.RuntimeDriverLocal, Mode: domain.RuntimeModeExecution}}
	request.RuntimeBindings = []domain.ResourceRuntimeBinding{{ResourceID: "resource", RuntimeID: "runtime", Enabled: true}}
	storage := &storageNavigatorStub{}
	store := &preflightCommandStub{}
	h := &Handler{executionCommands: store, storage: storage, connectionTest: func(context.Context, domain.EnvironmentConnection) ports.ConnectionHealth {
		return ports.ConnectionHealth{Healthy: true}
	}}
	report := h.evaluateExecutionPreflight(context.Background(), &request)
	if !report.Ready {
		t.Fatal(report)
	}
	if err := store.SavePreflight(context.Background(), report); err != nil {
		t.Fatal(err)
	}
	request.PreflightID, request.AcknowledgeWarnings = report.ID, true
	storage.err = errors.New("file removed after review")
	err := h.PrepareExecution(context.Background(), &request)
	if err == nil || !strings.Contains(err.Error(), "file removed after review") {
		t.Fatalf("stale input approval reused: %v", err)
	}
}

type imageProbeStub struct {
	BuildOrchestrator
	calls int
}

func (s *imageProbeStub) CheckExecutable(context.Context, domain.ExecutableSource) (bool, error) {
	s.calls++
	return false, errors.New("registry offline")
}
func TestPreflightReusesCatalogArtifactWithoutRegistryAccess(t *testing.T) {
	checker := &imageProbeStub{}
	h := &Handler{build: checker}
	request := preflightRequest()
	request.Workflow.Activities = []domain.Activity{{ID: "activity"}}
	request.PreparationRequirementsByActivity = map[string]domain.PreparationRequirement{
		"activity": {Artifact: &domain.ArtifactMaterialization{Digest: "verified-sif"}},
	}
	status, _, _ := h.preflightImages(&request)[0].Run(context.Background())
	if status != "unverified" || checker.calls != 0 {
		t.Fatalf("cached artifact incorrectly required registry: %s %d", status, checker.calls)
	}
}

func TestPreflightChecksEachPortableBuildOncePerReport(t *testing.T) {
	checker := &imageProbeStub{}
	h := &Handler{build: checker}
	request := preflightRequest()
	request.Workflow.Activities = nil
	for index := range 514 {
		archive := []byte("shared archive")
		if index == 513 {
			archive = []byte("different archive with the same declared digest")
		}
		request.Workflow.Activities = append(request.Workflow.Activities, domain.Activity{
			ID: strconv.Itoa(index),
			Command: domain.ActivityCommand{Executable: &domain.ExecutableReference{
				Source: domain.ExecutableSource{Origin: &domain.ArtifactOrigin{
					Build: &domain.PortableArtifactBuild{ContextArchive: archive, ContextDigest: "same"},
				}},
			}},
		})
	}
	for report := 1; report <= 2; report++ {
		probes := h.preflightImages(&request)
		if len(probes) != 514 {
			t.Fatalf("got %d activity checks, want 514", len(probes))
		}
		for index, probe := range probes {
			status, message, _ := probe.Run(context.Background())
			if probe.EntityID != strconv.Itoa(index) || status != "blocked" || message != "registry offline" {
				t.Fatalf("unexpected shared observation for activity %d: %s %s", index, status, message)
			}
		}
		if want := 2 * report; checker.calls != want {
			t.Fatalf("got %d executable lookups, want %d", checker.calls, want)
		}
	}
}

func TestConnectionProbeUsesRefreshedResourceConfiguration(t *testing.T) {
	request := preflightRequest()
	request.Plan.Assignments = []domain.PlanAssignment{{ActivityID: "a", ResourceID: "resource"}}
	request.Resources = []domain.Resource{{ID: "resource", Type: domain.ResourceLocalMachine}}
	h := &Handler{connectionTest: func(context.Context, domain.EnvironmentConnection) ports.ConnectionHealth {
		t.Fatal("stale local connection was probed")
		return ports.ConnectionHealth{}
	}}
	probes := h.preflightConnections(&request)
	request.Resources[0].Type = domain.ResourceHPCMachine
	status, _, _ := probes[0].Run(context.Background())
	if status != "unverified" {
		t.Fatalf("stale resource approved: %s", status)
	}
}

func TestPreflightDoesNotApproveStaleStorageCapacity(t *testing.T) {
	request := preflightRequest()
	request.Workflow.Activities = []domain.Activity{{ID: "activity", Metadata: map[string]any{
		"workspaceProjection": map[string]any{"contract": "whole-workspace-v1", "source": "measured", "inputs": []map[string]any{{"path": "input", "sizeBytes": 100, "contentId": "hash"}}},
	}}}
	request.Plan.Assignments = []domain.PlanAssignment{{ActivityID: "activity", ResourceID: "resource"}}
	request.Resources = []domain.Resource{{ID: "resource", Metadata: map[string]any{
		"workspaceStorageBudget": storagebudget.Snapshot{StorageID: "storage", AvailableBytes: 10000, Source: "quota-api", ObservedAt: time.Now().Add(-time.Hour)},
	}}}
	report := (&Handler{}).evaluateExecutionPreflight(context.Background(), &request)
	for _, check := range report.Checks {
		if check.Code == "storage_budget" {
			if check.Status != "unverified" || !strings.Contains(check.Message, "stale") {
				t.Fatalf("stale capacity approved: %+v", check)
			}
			return
		}
	}
	t.Fatal("missing storage check")
}
