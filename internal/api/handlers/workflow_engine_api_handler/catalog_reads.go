package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type environmentSummaryQuery interface {
	ListSummaries(context.Context, string, string) ([]domain.EnvironmentDefinition, error)
}

func (h *Handler) listWorkflowOptions(w http.ResponseWriter, r *http.Request) {
	query, ok := h.workflows.(interface {
		ListHeaders(context.Context) ([]domain.WorkflowDefinition, error)
	})
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("workflow options unavailable"))
		return
	}
	items, err := query.ListHeaders(r.Context())
	writeList(w, items, err)
}

func (h *Handler) countEnvironments(w http.ResponseWriter, r *http.Request) {
	query, ok := h.environments.(interface {
		Count(context.Context) (int, error)
	})
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("environment count unavailable"))
		return
	}
	total, err := query.Count(r.Context())
	writeList(w, map[string]int{"total": total}, err)
}

type filteredCloudOperations interface {
	ListCloudOperationsFiltered(context.Context, domain.CloudOperationFilter) ([]domain.CloudOperationRun, error)
}

func (h *Handler) listFilteredCloudOperations(w http.ResponseWriter, r *http.Request) {
	query, ok := h.cloudOperations.(filteredCloudOperations)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("filtered cloud operations unavailable"))
		return
	}
	limit := 0
	if r.URL.Query().Has("limit") {
		limit = positiveInteger(r.URL.Query().Get("limit"), 50)
		if limit > 500 {
			limit = 500
		}
	}
	items, err := query.ListCloudOperationsFiltered(r.Context(), domain.CloudOperationFilter{
		EnvironmentID: r.URL.Query().Get("environmentId"), InstanceID: r.URL.Query().Get("instanceId"),
		CapacityTargetID: r.URL.Query().Get("capacityTargetId"), ExecutionRunID: r.URL.Query().Get("executionRunId"), Limit: limit,
	})
	writeList(w, items, err)
}

func (h *Handler) listEnvironmentSummaries(w http.ResponseWriter, r *http.Request) {
	query, ok := h.environments.(environmentSummaryQuery)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("environment summaries unavailable"))
		return
	}
	items, err := query.ListSummaries(r.Context(), r.URL.Query().Get("executionScopeId"), r.URL.Query().Get("versionId"))
	writeList(w, items, err)
}

func (h *Handler) GetWorkflowVersion(w http.ResponseWriter, r *http.Request) {
	version, err := h.workflows.FindVersion(r.Context(), r.PathValue("versionId"))
	writeItem(w, version, err)
}

func (h *Handler) GetCloudInstance(w http.ResponseWriter, r *http.Request) {
	if h.cloud == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("cloud configuration unavailable"))
		return
	}
	instance, err := h.cloud.FindProvisionedInstance(r.Context(), r.PathValue("instanceId"))
	if instance != nil && err == nil {
		instance.Billing.RefreshCost(time.Now().UTC())
	}
	writeItem(w, instance, err)
}

func (h *Handler) listResourceInventory(w http.ResponseWriter, r *http.Request) {
	environments, ok := h.environments.(environmentSummaryQuery)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("environment summaries unavailable"))
		return
	}
	definitions, err := environments.ListSummaries(r.Context(), "", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	cloud := domain.CloudInventory{}
	if query, ok := h.cloud.(interface {
		Inventory(context.Context) (domain.CloudInventory, error)
	}); ok {
		cloud, err = query.Inventory(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	groups := make([]domain.ResourceInventoryGroup, 0, len(definitions))
	byEnvironment := make(map[string]int, len(definitions))
	for _, definition := range definitions {
		byEnvironment[definition.Environment.ID] = len(groups)
		groups = append(groups, domain.ResourceInventoryGroup{EnvironmentDefinition: definition})
	}
	for _, target := range cloud.Targets {
		if index, ok := byEnvironment[target.EnvironmentID]; ok {
			groups[index].CloudCapacityTargets = append(groups[index].CloudCapacityTargets, target)
		}
	}
	for _, instance := range cloud.Instances {
		if index, ok := byEnvironment[instance.EnvironmentID]; ok {
			groups[index].CloudInstances = append(groups[index].CloudInstances, instance)
		}
	}
	writeJSON(w, http.StatusOK, groups)
}
