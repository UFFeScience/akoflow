package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/domain"
	"gopkg.in/yaml.v3"
)

type reviewResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type reviewActivityEstimate struct {
	Name string `json:"name"`
	ActivityHistoryEstimate
}

type activityEstimateReview struct {
	Resources  []reviewResource         `json:"resources"`
	ResourceID string                   `json:"resourceId"`
	Activities []reviewActivityEstimate `json:"activities"`
}

func (m *Manager) serveActivityEstimateReview(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URI        string `json:"uri"`
		Hash       string `json:"hash"`
		ResourceID string `json:"resourceId"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	review, err := m.reviewActivityEstimates(r.Context(), input.URI, input.Hash, input.ResourceID)
	respond(w, review, err)
}

func (m *Manager) reviewActivityEstimates(ctx context.Context, uri, hash, selectedID string) (activityEstimateReview, error) {
	review := activityEstimateReview{Resources: []reviewResource{}, Activities: []reviewActivityEstimate{}}
	definition, err := m.WorkflowDefinition(ctx, uri)
	if err != nil {
		return review, err
	}
	if hash == "" || hash != definition.Hash {
		return review, failure(409, "workflow_source_changed", "Reload the workflow definition to review this version.")
	}
	var document any
	if err := yaml.Unmarshal([]byte(definition.YAML), &document); err != nil {
		return review, err
	}
	data, err := json.Marshal(document)
	if err != nil {
		return review, err
	}
	var request apirequests.Workflow
	if err := json.Unmarshal(data, &request); err != nil {
		return review, err
	}
	workflow, err := request.Domain()
	if err != nil {
		return review, err
	}
	var resources []domain.Resource
	var environments []domain.EnvironmentDefinition
	for _, read := range []struct {
		path   string
		target any
	}{{"/resources/", &resources}, {"/environments/", &environments}} {
		data, err := m.sourceRequest(ctx, http.MethodGet, read.path, nil, "")
		if err != nil {
			return review, err
		}
		if err := json.Unmarshal(data, read.target); err != nil {
			return review, err
		}
	}
	sort.SliceStable(resources, func(i, j int) bool {
		if resources[i].Name == resources[j].Name {
			return resources[i].ID < resources[j].ID
		}
		return resources[i].Name < resources[j].Name
	})
	var selected domain.Resource
	for _, resource := range resources {
		if !resource.Schedulable || historyArchitecture(resource.Architecture) == "" || historyRuntime(resource.ID, environments) == "" {
			continue
		}
		review.Resources = append(review.Resources, reviewResource{ID: resource.ID, Name: resource.Name})
		if resource.ID == selectedID || selectedID == "" && selected.ID == "" {
			selected = resource
		}
	}
	if selected.ID == "" || definition.PublicationID == "" {
		return review, nil
	}
	review.ResourceID = selected.ID
	model, _ := selected.Metadata["cpuModel"].(string)
	if !cpuModel.MatchString(model) {
		model = ""
	}
	profiles := historyProfilesForResource(workflow.Version, selected, historyArchitecture(selected.Architecture), historyRuntime(selected.ID, environments), model)
	estimates, err := m.activityEstimates(ctx, historySourceURI(definition.PublicationID, definition.Version), definition.origin, profiles)
	if err != nil {
		return review, err
	}
	for _, activity := range workflow.Version.Activities {
		if estimate, ok := estimates[historyActivityID(activity)]; ok {
			review.Activities = append(review.Activities, reviewActivityEstimate{Name: activity.Name, ActivityHistoryEstimate: estimate})
		}
	}
	return review, nil
}
