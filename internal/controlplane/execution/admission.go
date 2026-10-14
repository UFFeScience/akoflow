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

// Preserve intentional overcommit for unrestricted legacy resources. Cloud VMs
// honor explicit capacity even when it equals the physical core count; other
// resources honor an explicit reduction of the physical lane capacity.
func restrictedCapacityReady(ready []string, assignments map[string]domain.PlanAssignment, resources map[string]domain.Resource, tasks map[string]domain.TaskExecution, pending map[string]bool) []string {
	occupied := make(map[string]int)
	for id, task := range tasks {
		if task.Status == domain.TaskPreparing || task.Status == domain.TaskRunning || pending[id] {
			occupied[assignments[id].ResourceID]++
		}
	}
	eligible := make([]string, 0, len(ready))
	for _, id := range ready {
		resourceID := assignments[id].ResourceID
		r := resources[resourceID]
		if r.CPUCapacity > 0 && (r.Type == domain.ResourceCloudVM || r.CPUCapacity < float64(r.CPUCores)) {
			if occupied[resourceID] >= r.SchedulingCPUCores() {
				continue
			}
			occupied[resourceID]++
		}
		eligible = append(eligible, id)
	}
	return eligible
}
