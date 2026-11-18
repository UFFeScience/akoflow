package workflow_engine_api_handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/application/recurring"
	executioncontrol "github.com/UFFeScience/akoflow/internal/controlplane/execution"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

type executionEnvironmentReader interface {
	ExecutionDefinition(context.Context, string) (*domain.EnvironmentDefinition, error)
}

func (h *Handler) WithExecutionSchedules(store recurring.Store) *recurring.Coordinator {
	h.executionSchedules = &recurring.Coordinator{Store: store, Request: h.scheduleExecutionRequest}
	return h.executionSchedules
}

func (h *Handler) ListExecutionSchedules(w http.ResponseWriter, r *http.Request) {
	if h.executionSchedules == nil {
		writeJSON(w, http.StatusOK, []domainexecution.Schedule{})
		return
	}
	values, err := h.executionSchedules.Store.ListSchedules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (h *Handler) CreateExecutionSchedule(w http.ResponseWriter, r *http.Request) {
	if h.executionSchedules == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("execution scheduling is unavailable"))
		return
	}
	var input domainexecution.Schedule
	if !decode(w, r, &input) {
		return
	}
	schedule, err := h.executionSchedules.Create(r.Context(), input, time.Now().UTC())
	if err != nil {
		writeExecutionScheduleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, schedule)
}

func (h *Handler) UpdateExecutionSchedule(w http.ResponseWriter, r *http.Request) {
	if h.executionSchedules == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("execution scheduling is unavailable"))
		return
	}
	var input struct {
		Paused *bool `json:"paused"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Paused == nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("paused is required"))
		return
	}
	schedule, err := h.executionSchedules.SetPaused(r.Context(), r.PathValue("scheduleId"), *input.Paused, time.Now().UTC())
	if err != nil {
		writeExecutionScheduleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, schedule)
}

func writeExecutionScheduleError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, recurring.ErrConfiguration) {
		status = http.StatusUnprocessableEntity
	}
	if errors.Is(err, recurring.ErrNotFound) {
		status = http.StatusNotFound
	}
	writeError(w, status, err)
}

func (h *Handler) scheduleExecutionRequest(ctx context.Context, schedule domainexecution.Schedule) (ports.ExecutionRequest, error) {
	var request ports.ExecutionRequest
	plan, err := h.plans.Find(ctx, schedule.PlanID)
	if err != nil {
		return request, err
	}
	if plan == nil || plan.WorkflowVersionID != schedule.WorkflowVersionID {
		return request, fmt.Errorf("%w: the plan must belong to the pinned workflow version", recurring.ErrConfiguration)
	}
	workflow, err := h.workflows.FindVersion(ctx, schedule.WorkflowVersionID)
	if err != nil {
		return request, err
	}
	if workflow == nil || workflow.Archived {
		return request, fmt.Errorf("%w: the pinned workflow version is unavailable or archived", recurring.ErrConfiguration)
	}
	scope, err := h.scopes.FindScope(ctx, plan.ExecutionScopeID)
	if err != nil {
		return request, err
	}
	if scope == nil || len(scope.EnvironmentVersionIDs) == 0 {
		return request, fmt.Errorf("%w: the plan has no available execution scope", recurring.ErrConfiguration)
	}
	request.Plan, request.Workflow, request.ExecutionScope = *plan, *workflow, *scope
	if plan.NetworkTopologyID != "" {
		topology, err := h.topologies.Find(ctx, plan.NetworkTopologyID)
		if err != nil {
			return request, err
		}
		if topology == nil {
			return request, fmt.Errorf("%w: the plan topology is unavailable", recurring.ErrConfiguration)
		}
		request.NetworkTopology = *topology
	}
	if err := h.loadScheduleEnvironments(ctx, &request); err != nil {
		return request, err
	}
	mode := domain.ExecutionModeReal
	if target, _ := plan.Metadata["targetMode"].(string); target == "simulation" ||
		(hasScheduleRuntime(request.Runtimes, "simulation") && !hasScheduleRuntime(request.Runtimes, "execution")) {
		mode = domain.ExecutionModeSimulation
	}
	request.Run = domain.ExecutionRun{ID: schedule.ID, SchedulePlanID: plan.ID, Status: domain.ExecutionRunCreated, Mode: mode}
	if len(request.Plan.Assignments) == 0 {
		return request, fmt.Errorf("%w: the plan has no activities", recurring.ErrConfiguration)
	}
	if err := executioncontrol.ValidateRequest(request); err != nil {
		return request, fmt.Errorf("%w: %v", recurring.ErrConfiguration, err)
	}
	resolveDirectOCIImages(&request.Workflow)
	return request, nil
}

func (h *Handler) loadScheduleEnvironments(ctx context.Context, request *ports.ExecutionRequest) error {
	environments, ok := h.environments.(executionEnvironmentReader)
	if !ok {
		return fmt.Errorf("versioned execution environment lookup is unavailable")
	}
	for _, version := range request.ExecutionScope.EnvironmentVersionIDs {
		definition, err := environments.ExecutionDefinition(ctx, version)
		if err != nil {
			return err
		}
		if definition == nil {
			return fmt.Errorf("%w: a pinned environment version is unavailable", recurring.ErrConfiguration)
		}
		request.Resources = append(request.Resources, definition.Resources...)
		request.Runtimes = append(request.Runtimes, definition.Runtimes...)
		request.RuntimeBindings = append(request.RuntimeBindings, definition.RuntimeBindings...)
		request.ActivityProfiles = append(request.ActivityProfiles, definition.Profiles...)
	}
	return nil
}

func hasScheduleRuntime(runtimes []domain.EnvironmentRuntime, mode string) bool {
	for _, runtime := range runtimes {
		if string(runtime.Mode) == mode {
			return true
		}
	}
	return false
}
