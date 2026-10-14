package workflow

import "math"

func (profile ActivityResourceProfile) UsesWorkUnits() bool {
	return profile.ModelVersion == "reference-work-v1" || profile.ModelVersion == "compute-work-v1"
}

// reference-work-v1 uses the existing persisted ModelVersion discriminator:
// RuntimeSeconds is seconds of measured wall time per one reference second.
// It must not be treated as an absolute per-activity replay after API reload.
func (profile ActivityResourceProfile) RuntimeFor(activity Activity) float64 {
	if !profile.UsesWorkUnits() {
		return profile.RuntimeSeconds
	}
	work := 0.0
	if profile.ModelVersion == "compute-work-v1" {
		work, _ = activity.Metadata["computeWorkUnits"].(float64)
	} else if activity.Simulation != nil {
		work = activity.Simulation.DurationSeconds
	}
	seconds := profile.RuntimeSeconds * work
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return 0
	}
	return seconds
}
