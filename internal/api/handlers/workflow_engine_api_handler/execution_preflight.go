package workflow_engine_api_handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	apppreflight "github.com/UFFeScience/akoflow/internal/application/preflight"
	executioncontrol "github.com/UFFeScience/akoflow/internal/controlplane/execution"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainpreflight "github.com/UFFeScience/akoflow/internal/domain/preflight"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
	"github.com/UFFeScience/akoflow/internal/operationerror"
	"github.com/google/uuid"
)

type preflightStore interface {
	SavePreflight(context.Context, domainpreflight.Report) error
	FindPreflight(context.Context, string) (*domainpreflight.Report, error)
}

func executionContextHash(request ports.ExecutionRequest) string {
	request.PreflightID, request.AcknowledgeWarnings = "", false
	payload, _ := json.Marshal(request)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (h *Handler) CreateExecutionPreflight(w http.ResponseWriter, r *http.Request) {
	var request ports.ExecutionRequest
	if !decode(w, r, &request) {
		return
	}
	store, ok := h.executionCommands.(preflightStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("preflight persistence is unavailable"))
		return
	}
	report := h.evaluateExecutionPreflight(r.Context(), &request)
	if err := store.SavePreflight(r.Context(), report); err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) GetExecutionPreflight(w http.ResponseWriter, r *http.Request) {
	store, ok := h.executionCommands.(preflightStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("preflight persistence is unavailable"))
		return
	}
	report, err := store.FindPreflight(r.Context(), r.PathValue("preflightId"))
	writeItem(w, report, err)
}

func (h *Handler) evaluateExecutionPreflight(ctx context.Context, request *ports.ExecutionRequest) domainpreflight.Report {
	return h.evaluateExecutionPrerequisites(ctx, request, true)
}

func (h *Handler) evaluateExecutionPrerequisites(ctx context.Context, request *ports.ExecutionRequest, inspectImages bool) domainpreflight.Report {
	hash := executionContextHash(*request)
	probe := func(code, stage string, check func(context.Context) error) apppreflight.Probe {
		return apppreflight.Probe{Code: code, Stage: stage, Run: func(ctx context.Context) (string, string, string) {
			if err := check(ctx); err != nil {
				return "blocked", err.Error(), operationerror.Describe(err).NextAction
			}
			return "passed", "Configured prerequisites are valid.", ""
		}}
	}
	probes := []apppreflight.Probe{
		probe("execution_configuration", "configuration", func(context.Context) error { return executioncontrol.ValidateRequest(*request) }),
		probe("local_allocation", "resources", func(ctx context.Context) error { return h.resolveLocalExecutionAllocation(ctx, request) }),
	}
	if request.Run.Mode == domain.ExecutionModeSimulation {
		probes = append(probes, apppreflight.Probe{Code: "simulation_only", Stage: "mode", Run: func(context.Context) (string, string, string) {
			return "warning", "Simulation validates configuration only; it does not prove real image, storage or connection access.", "Use real execution validation before running on infrastructure."
		}})
		return apppreflight.Evaluate(ctx, "preflight-"+uuid.NewString(), hash, string(request.Run.Mode), probes)
	}
	probes = append(probes,
		probe("workspace_access", "storage", func(ctx context.Context) error { return h.resolveWorkspaceHomes(ctx, request) }),
		apppreflight.Probe{Code: "storage_budget", Stage: "storage", Run: func(context.Context) (string, string, string) {
			resources, err := preflightStorageResources(*request)
			if err != nil {
				return "blocked", err.Error(), "Review workspace storage configuration."
			}
			timings := []storagebudget.Timing{}
			for _, assignment := range request.Plan.Assignments {
				timings = append(timings, storagebudget.Timing{ActivityID: assignment.ActivityID, ResourceID: assignment.ResourceID, Start: assignment.PredictedStartAt, Finish: assignment.PredictedFinishAt})
			}
			reservations, missing, err := storagebudget.CheckScheduled(request.Workflow, resources, timings)
			if err != nil {
				return "blocked", err.Error(), "Free space or select a workspace with sufficient capacity."
			}
			if len(missing) > 0 || len(reservations) == 0 {
				return "unverified", "A measured storage budget is unavailable for one or more destinations.", "Check destination capacity before starting; runtime storage limits remain enforced."
			}
			for _, reservation := range reservations {
				age := time.Since(reservation.ObservedAt)
				if age > 2*time.Minute || age < -30*time.Second {
					return "unverified", "Storage capacity observations are stale or have an invalid observation time.", "Refresh destination storage capacity before starting."
				}
			}
			return "passed", "Declared workspace requirements fit the observed storage budgets.", ""
		}},
		probe("artifact_compatibility", "artifacts", func(ctx context.Context) error { return h.resolveBuildPreparations(ctx, request) }),
	)
	probes = append(probes, h.preflightConnections(request)...)
	probes = append(probes, h.preflightInputs(*request)...)
	if inspectImages {
		probes = append(probes, h.preflightImages(request)...)
	}
	return apppreflight.Evaluate(ctx, "preflight-"+uuid.NewString(), hash, string(request.Run.Mode), probes)
}

func (h *Handler) PrepareExecution(ctx context.Context, request *ports.ExecutionRequest) error {
	report := h.evaluateExecutionPrerequisites(ctx, request, false)
	if store, ok := h.executionCommands.(preflightStore); ok {
		if err := store.SavePreflight(ctx, report); err != nil {
			return fmt.Errorf("persist launch validation: %w", err)
		}
	}
	if !report.Ready {
		for _, check := range report.Checks {
			if check.Status == "blocked" {
				return operationerror.Wrap(fmt.Errorf("%s", check.Message), check.Code, check.Stage, check.EntityID, false, check.NextAction)
			}
		}
	}
	return nil
}
