package workflow

import "math"

// reference-work-v1 uses the existing persisted ModelVersion discriminator:
// RuntimeSeconds is seconds of measured wall time per one reference second.
// It must not be treated as an absolute per-activity replay after API reload.
func (profile ActivityResourceProfile) RuntimeFor(activity Activity) float64 {
	if profile.ModelVersion != "reference-work-v1" {
		return profile.RuntimeSeconds
	}
	if activity.Simulation == nil || activity.Simulation.DurationSeconds <= 0 {
		return 0
	}
	seconds := profile.RuntimeSeconds * activity.Simulation.DurationSeconds
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return 0
	}
	return seconds
}
