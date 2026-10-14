package simgrid

import (
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	"math"
)

// Reject success responses from a runner that silently ignores requested
// stages. Older runners remain compatible when no admission delay is requested.
func validateAdmissionTrace(request ports.ExecutionRequest, model *materializationmodel.Model, trace domain.ExecutionTrace) error {
	if model == nil {
		return nil
	}
	expected := map[string]float64{}
	for _, a := range request.Plan.Assignments {
		delay, err := model.AdmissionSeconds(a.ResourceID)
		if err != nil {
			return err
		}
		if delay > 0 {
			expected[a.ActivityID] = delay
		}
	}
	seen := map[string]bool{}
	for _, t := range trace.Tasks {
		want, required := expected[t.ActivityID]
		if !required {
			continue
		}
		got, ok := numberMetadata(t.Metadata, "externalAdmissionSeconds")
		if seen[t.ActivityID] || !ok || math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 1e-8*math.Max(1, want) {
			return fmt.Errorf("SimGrid trace does not acknowledge requested admission stage for %s", t.ActivityID)
		}
		seen[t.ActivityID] = true
	}
	for id := range expected {
		if !seen[id] {
			return fmt.Errorf("SimGrid trace is missing admission stage for %s", id)
		}
	}
	return nil
}
