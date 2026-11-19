package hubpublish

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type ExecutionReference struct {
	ExperimentID string `json:"experimentId"`
	RunID        string `json:"runId"`
}

type executionReferenceChoice struct {
	Path       string `json:"path,omitempty"`
	Activities []struct {
		Name    string  `json:"name"`
		Seconds float64 `json:"seconds"`
	} `json:"activities,omitempty"`
	ExecutionReference
	Author          string  `json:"author"`
	Date            string  `json:"date"`
	MachineLabel    string  `json:"machineLabel,omitempty"`
	DurationSeconds float64 `json:"durationSeconds"`
	Resources       []struct {
		Architecture   string `json:"architecture"`
		CPUModel       string `json:"cpuModel,omitempty"`
		CPUCores       int    `json:"cpuCores"`
		MemoryBytes    int64  `json:"memoryBytes"`
		OS             string `json:"os"`
		Runtime        string `json:"runtime"`
		RuntimeVersion string `json:"runtimeVersion,omitempty"`
	} `json:"resources"`
}

type ActivityHistoryProfile struct {
	ImageDigest     string  `json:"imageDigest,omitempty"`
	Platform        string  `json:"platform,omitempty"`
	Architecture    string  `json:"architecture"`
	Runtime         string  `json:"runtime"`
	CPUModel        string  `json:"cpuModel,omitempty"`
	CPUCores        float64 `json:"cpuCores,omitempty"`
	MemoryBytes     float64 `json:"memoryBytes,omitempty"`
	AllocatedCPU    float64 `json:"allocatedCpu,omitempty"`
	AllocatedMemory float64 `json:"allocatedMemory,omitempty"`
	InputBytes      int64   `json:"inputBytes,omitempty"`
}

type ActivityHistoryEstimate struct {
	SelectedReference        bool     `json:"selectedReference,omitempty"`
	MedianSeconds            float64  `json:"medianSeconds"`
	MinSeconds               float64  `json:"minSeconds"`
	MaxSeconds               float64  `json:"maxSeconds"`
	SampleCount              int      `json:"sampleCount"`
	PreparationMedianSeconds *float64 `json:"preparationMedianSeconds,omitempty"`
}

func (m *Manager) ActivityEstimates(ctx context.Context, uri string, activities map[string]ActivityHistoryProfile) (map[string]ActivityHistoryEstimate, error) {
	return m.activityEstimates(ctx, uri, "", activities, nil)
}

func (m *Manager) activityEstimates(ctx context.Context, uri, expectedOrigin string, activities map[string]ActivityHistoryProfile, reference *ExecutionReference) (map[string]ActivityHistoryEstimate, error) {
	publicationID, version, err := parseWorkflowURI(uri)
	if err != nil {
		return nil, err
	}
	estimates := make(map[string]ActivityHistoryEstimate)
	if len(activities) == 0 {
		return estimates, nil
	}
	if len(activities) > 256 {
		return nil, failure(422, "invalid_activity_estimates", "Request estimates for up to 256 activities at a time.")
	}
	path := fmt.Sprintf("/publications/%s/versions/%d/activity-estimates", publicationID, version)
	body := struct {
		Activities map[string]ActivityHistoryProfile `json:"activities"`
		Reference  *ExecutionReference               `json:"reference,omitempty"`
	}{activities, reference}
	if err := m.historyRequest(ctx, expectedOrigin, http.MethodPost, path, body, &estimates); err != nil {
		return nil, err
	}
	if estimates == nil {
		estimates = make(map[string]ActivityHistoryEstimate)
	}
	minimum := 3
	if reference != nil {
		minimum = 1
	}
	for id, estimate := range estimates {
		if _, requested := activities[id]; !requested || estimate.SampleCount < minimum || estimate.SelectedReference != (reference != nil) ||
			estimate.MinSeconds <= 0 || estimate.MedianSeconds < estimate.MinSeconds || estimate.MaxSeconds < estimate.MedianSeconds {
			delete(estimates, id)
		}
	}
	return estimates, nil
}

func (m *Manager) historyRequest(ctx context.Context, expectedOrigin, method, path string, body, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if expectedOrigin != "" && m.state.HubURL != expectedOrigin {
		return nil
	}
	err := m.requestLocked(ctx, method, path, body, nil, false, result)
	var response *apiError
	if errors.As(err, &response) && response.Status == http.StatusNotFound && m.state.AccessToken != "" {
		err = m.requestLocked(ctx, method, "/desktop"+path, body, nil, true, result)
	}
	return err
}

func (m *Manager) serveActivityEstimates(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URI        string                            `json:"uri"`
		Activities map[string]ActivityHistoryProfile `json:"activities"`
	}
	if err := decodeBoundedRequest(w, r, &input, 256<<10); err != nil {
		respond(w, nil, err)
		return
	}
	estimates, err := m.ActivityEstimates(r.Context(), input.URI, input.Activities)
	respond(w, estimates, err)
}
