package planning

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// A planning session freezes the measured profile entities independently of
// mutable catalog profiles, just as it freezes interference knowledge. This
// avoids replacing an environment and destroying its execution provenance.
func calibratedProfileSnapshot(raw any, workflow domain.WorkflowVersion, resources []domain.Resource) ([]domain.ActivityResourceProfile, error) {
	if raw == nil {
		return nil, nil
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var profiles []domain.ActivityResourceProfile
	if err = json.Unmarshal(payload, &profiles); err != nil {
		return nil, fmt.Errorf("decode activity profile snapshot: %w", err)
	}
	types, ids, pairs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, activity := range workflow.Activities {
		if activity.Simulation == nil || activity.Simulation.DurationSeconds <= 0 {
			return nil, fmt.Errorf("reference-work snapshot requires positive activity reference work: %s", activity.ID)
		}
		types[activity.ActivityTypeID] = true
	}
	for _, resource := range resources {
		ids[resource.ID] = true
	}
	for _, profile := range profiles {
		pair := profile.ActivityTypeID + "\x00" + profile.ResourceID
		if !types[profile.ActivityTypeID] || !ids[profile.ResourceID] || pairs[pair] || profile.ID == "" || profile.Source == "" || profile.SampleSize < 1 || profile.ModelVersion != "reference-work-v1" || profile.RuntimeSeconds <= 0 || math.IsInf(profile.RuntimeSeconds, 0) || math.IsNaN(profile.RuntimeSeconds) {
			return nil, fmt.Errorf("invalid or duplicate calibrated profile %q", profile.ID)
		}
		pairs[pair] = true
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("empty calibrated profile snapshot")
	}
	return profiles, nil
}
