package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// GetPlanContext resolves the persisted ownership/version/scope, without
// downloading global workflow, resource, environment and topology catalogs.
func (h *Handler) GetPlanContext(w http.ResponseWriter, r *http.Request) {
	plan, err := h.plans.Find(r.Context(), r.PathValue("planId"))
	if err != nil || plan == nil {
		writeItem(w, plan, err)
		return
	}
	workflow, err := h.planWorkflow(r.Context(), plan.WorkflowVersionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	query, ok := h.environments.(environmentSummaryQuery)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("scoped environment summaries unavailable"))
		return
	}
	environments := []domain.EnvironmentDefinition{}
	if plan.ExecutionScopeID != "" {
		environments, err = query.ListSummaries(r.Context(), plan.ExecutionScopeID, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	resources := make([]domain.Resource, 0)
	for _, environment := range environments {
		resources = append(resources, environment.Resources...)
	}
	var topology *domain.NetworkTopology
	if plan.NetworkTopologyID != "" {
		topology, err = h.topologies.Find(r.Context(), plan.NetworkTopologyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plan": plan, "workflow": workflow, "environments": environments, "resources": resources, "topology": topology,
	})
}

func (h *Handler) planWorkflow(ctx context.Context, versionID string) (*domain.WorkflowDefinition, error) {
	version, err := h.workflows.FindVersion(ctx, versionID)
	if err != nil || version == nil {
		return nil, err
	}
	definition := &domain.WorkflowDefinition{ID: version.WorkflowID, Name: version.WorkflowID}
	if query, ok := h.workflows.(interface {
		FindHeader(context.Context, string) (*domain.WorkflowDefinition, error)
	}); ok {
		definition, err = query.FindHeader(ctx, version.WorkflowID)
		if err != nil || definition == nil {
			return nil, err
		}
	}
	definition.Version = *version
	return definition, nil
}
