package workflow

import (
	"fmt"
	"math"
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain/resource"
)

// CompleteSpeedupProfiles preserves measured pairs and fills only missing pairs.
// Estimates retain the donor's work-unit model, never claim measured samples,
// and do not remove resource/runtime or materialization compatibility checks.
func CompleteSpeedupProfiles(activities []Activity, resources []resource.Resource, profiles []ActivityResourceProfile) ([]ActivityResourceProfile, error) {
	valid := func(v float64) bool { return v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) }
	byID := map[string]resource.Resource{}
	for _, r := range resources {
		byID[r.ID] = r
	}
	pairs := map[string]bool{}
	donors := map[string][]ActivityResourceProfile{}
	for _, p := range profiles {
		pairs[p.ActivityTypeID+"\x00"+p.ResourceID] = true
		if p.UsesWorkUnits() && valid(p.RuntimeSeconds) && valid(byID[p.ResourceID].ComputeSpeedup) && p.SampleSize > 0 {
			donors[p.ActivityTypeID] = append(donors[p.ActivityTypeID], p)
		}
	}
	for family := range donors {
		sort.Slice(donors[family], func(i, j int) bool {
			a, b := donors[family][i], donors[family][j]
			if a.SampleSize != b.SampleSize {
				return a.SampleSize > b.SampleSize
			}
			return a.ID < b.ID
		})
	}
	out := append([]ActivityResourceProfile(nil), profiles...)
	for _, a := range activities {
		for _, r := range resources {
			key := a.ActivityTypeID + "\x00" + r.ID
			if !r.Schedulable || pairs[key] {
				continue
			}
			if !valid(r.ComputeSpeedup) || len(donors[a.ActivityTypeID]) == 0 {
				return nil, fmt.Errorf("speedup estimate requires measured work-unit donor and positive speedup: %s/%s", a.ActivityTypeID, r.ID)
			}
			d := donors[a.ActivityTypeID][0]
			sourceSpeed := byID[d.ResourceID].ComputeSpeedup
			seconds := d.RuntimeSeconds * sourceSpeed / r.ComputeSpeedup
			if !valid(seconds) {
				return nil, fmt.Errorf("invalid speedup estimate: %s", key)
			}
			out = append(out, ActivityResourceProfile{ID: "speedup-estimate-" + a.ActivityTypeID + "-" + r.ID, ActivityTypeID: a.ActivityTypeID, ResourceID: r.ID, RuntimeSeconds: seconds, ModelVersion: d.ModelVersion, Source: "speedup-estimate-v1:" + d.ID, Metadata: map[string]any{
				"estimated": true, "calibrationStatus": "extrapolated-not-measured", "donorProfileId": d.ID, "donorSource": d.Source, "donorSampleSize": d.SampleSize, "sourceSpeedup": sourceSpeed, "targetSpeedup": r.ComputeSpeedup, "formula": "donorSecondsPerWorkUnit * sourceSpeedup / targetSpeedup",
			}})
			pairs[key] = true
		}
	}
	return out, nil
}
