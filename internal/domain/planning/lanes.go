package planning

import "sort"

// LanePredecessors preserves the selected schedule within each resource/core/slot.
// Equal legacy order values retain their order in the persisted assignment list.
func LanePredecessors(assignments []PlanAssignment) map[string]string {
	byLane := make(map[string][]PlanAssignment)
	for _, assignment := range assignments {
		key := assignment.ResourceID + "\x00" + assignment.CoreID + "\x00" + assignment.SlotID
		byLane[key] = append(byLane[key], assignment)
	}
	previous := make(map[string]string)
	for _, lane := range byLane {
		sort.SliceStable(lane, func(i, j int) bool {
			return lane[i].OrderOnResource < lane[j].OrderOnResource
		})
		for i := 1; i < len(lane); i++ {
			previous[lane[i].ActivityID] = lane[i-1].ActivityID
		}
	}
	return previous
}
