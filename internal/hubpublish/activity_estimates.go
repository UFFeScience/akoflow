package hubpublish

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

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
	MedianSeconds float64 `json:"medianSeconds"`
	MinSeconds    float64 `json:"minSeconds"`
	MaxSeconds    float64 `json:"maxSeconds"`
	SampleCount   int     `json:"sampleCount"`
}

func (m *Manager) ActivityEstimates(ctx context.Context, uri string, activities map[string]ActivityHistoryProfile) (map[string]ActivityHistoryEstimate, error) {
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
	}{activities}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	err = m.requestLocked(ctx, http.MethodPost, path, body, nil, false, &estimates)
	var response *apiError
	if errors.As(err, &response) && response.Status == http.StatusNotFound && m.state.AccessToken != "" {
		err = m.requestLocked(ctx, http.MethodPost, "/desktop"+path, body, nil, true, &estimates)
	}
	if err != nil {
		return nil, err
	}
	if estimates == nil {
		estimates = make(map[string]ActivityHistoryEstimate)
	}
	for id, estimate := range estimates {
		if _, requested := activities[id]; !requested || estimate.SampleCount < 3 ||
			estimate.MinSeconds <= 0 || estimate.MedianSeconds < estimate.MinSeconds || estimate.MaxSeconds < estimate.MedianSeconds {
			delete(estimates, id)
		}
	}
	return estimates, nil
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
