package execution

import (
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Plan order is a scheduling preference, never an extra dependency edge.
func plannedReadyActivities(
	ready []string,
	assignments map[string]domain.PlanAssignment,
	pending map[string]bool,
) []string {
	ordered := append([]string(nil), ready...)
	sort.SliceStable(ordered, func(left, right int) bool {
		a, b := assignments[ordered[left]], assignments[ordered[right]]
		if a.PredictedStartAt != b.PredictedStartAt {
			return a.PredictedStartAt < b.PredictedStartAt
		}
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if a.OrderOnResource != b.OrderOnResource {
			return a.OrderOnResource < b.OrderOnResource
		}
		return ordered[left] < ordered[right]
	})
	eligible := make([]string, 0, len(ordered))
	for _, activityID := range ordered {
		if pending[activityID] {
			continue
		}
		eligible = append(eligible, activityID)
	}
	return eligible
}
