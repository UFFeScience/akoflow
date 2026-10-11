// Package storagebudget checks explicit effective quota snapshots, not nominal
// filesystem capacity. It does not assume cleanup or deduplication across
// separate workspaces. Reservations are conservative bounds, not measured peaks.
package storagebudget

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type Snapshot struct {
	StorageID      string    `json:"storageId"`
	AvailableBytes int64     `json:"availableBytes"`
	ObservedAt     time.Time `json:"observedAt"`
	Source         string    `json:"source"`
}

type Reservation struct {
	StorageID               string    `json:"storageId"`
	AvailableBytes          int64     `json:"availableBytes"`
	ReservedUpperBoundBytes int64     `json:"reservedUpperBoundBytes"`
	Source                  string    `json:"source"`
	ObservedAt              time.Time `json:"observedAt"`
}

// InsufficientQuota is candidate infeasibility, not a malformed model.
type InsufficientQuota struct{ Reservation Reservation }

func (e *InsufficientQuota) Error() string {
	r := e.Reservation
	return fmt.Sprintf("workspace storage quota %s insufficient: available %d bytes, conservative reservation %d bytes; verified cleanup/export or more quota required", r.StorageID, r.AvailableBytes, r.ReservedUpperBoundBytes)
}

type Timing struct {
	ActivityID string
	ResourceID string
	Start      float64
	Finish     float64
}

// CheckScheduled credits producer release only after every consumer's command
// barrier. Staging is still reserved for the entire run: no unproved archive
// cleanup savings are claimed. This is a forecast bound, not a live disk lease.
func CheckScheduled(version workflow.WorkflowVersion, resources []resource.Resource, timings []Timing) ([]Reservation, []string, error) {
	assignments := map[string]string{}
	byActivity := map[string]Timing{}
	for _, t := range timings {
		if t.ActivityID == "" || math.IsNaN(t.Start) || math.IsNaN(t.Finish) || math.IsInf(t.Start, 0) || math.IsInf(t.Finish, 0) || t.Start < 0 || t.Finish < t.Start {
			return nil, nil, fmt.Errorf("invalid storage schedule for %s", t.ActivityID)
		}
		if _, exists := byActivity[t.ActivityID]; exists {
			return nil, nil, fmt.Errorf("duplicate storage schedule activity %s", t.ActivityID)
		}
		assignments[t.ActivityID], byActivity[t.ActivityID] = t.ResourceID, t
	}
	for _, a := range version.Activities {
		if _, exists := byActivity[a.ID]; !exists {
			return nil, nil, fmt.Errorf("missing storage schedule activity %s", a.ID)
		}
	}
	return check(version, resources, assignments, byActivity)
}

// Check projects runtime-specific manifests and reserves all workspaces plus
// packed/received input staging until run end. This deliberately does not grant
// early-release savings unless the execution contract proves those releases.
// Shared logical nodes count against one identical effective quota snapshot.
// Missing budgets are returned explicitly; legacy workflows remain compatible.
func Check(version workflow.WorkflowVersion, resources []resource.Resource, assignments map[string]string) ([]Reservation, []string, error) {
	return check(version, resources, assignments, nil)
}

func check(version workflow.WorkflowVersion, resources []resource.Resource, assignments map[string]string, timings map[string]Timing) ([]Reservation, []string, error) {
	byID := map[string]resource.Resource{}
	types := map[string]string{}
	budgets := map[string]Snapshot{}
	for _, r := range resources {
		byID[r.ID] = r
	}
	missing := map[string]bool{}
	for activity, id := range assignments {
		r, ok := byID[id]
		if !ok {
			return nil, nil, fmt.Errorf("storage budget: unknown resource %s", id)
		}
		types[activity] = string(r.Type)
		raw := r.Metadata["workspaceStorageBudget"]
		if raw == nil {
			missing[id] = true
			continue
		}
		payload, err := json.Marshal(raw)
		if err != nil {
			return nil, nil, err
		}
		var b Snapshot
		if err := json.Unmarshal(payload, &b); err != nil {
			return nil, nil, fmt.Errorf("storage budget %s: %w", id, err)
		}
		if b.StorageID == "" || b.Source == "" || b.ObservedAt.IsZero() || b.AvailableBytes < 0 {
			return nil, nil, fmt.Errorf("storage budget %s requires storage identity, source, observation and nonnegative available bytes", id)
		}
		if previous, ok := budgets[b.StorageID]; ok && previous != b {
			return nil, nil, fmt.Errorf("inconsistent quota snapshots for shared storage %s", b.StorageID)
		}
		budgets[b.StorageID] = b
	}
	missingIDs := []string{}
	for id := range missing {
		missingIDs = append(missingIDs, id)
	}
	sort.Strings(missingIDs)
	if len(budgets) == 0 {
		return nil, missingIDs, nil
	}
	// The execution synchronizes both control and explicit data predecessors.
	// Include data-only edges in the projection as well as in lease timing.
	projectionVersion := version
	projectionVersion.Dependencies = append([]workflow.ActivityDependency(nil), version.Dependencies...)
	seenEdges := map[[2]string]bool{}
	for _, d := range projectionVersion.Dependencies {
		seenEdges[[2]string{d.ActivityID, d.DependsOnActivityID}] = true
	}
	for _, d := range version.DataDependencies {
		key := [2]string{d.ConsumerActivityID, d.ProducerActivityID}
		if !seenEdges[key] {
			projectionVersion.Dependencies = append(projectionVersion.Dependencies, workflow.ActivityDependency{ActivityID: key[0], DependsOnActivityID: key[1]})
			seenEdges[key] = true
		}
	}
	projected, err := workflow.ProjectWorkspaceDependenciesForResourceTypes(projectionVersion, types)
	if err != nil {
		return nil, missingIDs, err
	}
	reserved := map[string]int64{}
	edges := append([]workflow.ActivityDependency(nil), version.Dependencies...)
	for _, d := range version.DataDependencies {
		edges = append(edges, workflow.ActivityDependency{ActivityID: d.ConsumerActivityID, DependsOnActivityID: d.ProducerActivityID})
	}
	type event struct {
		at    float64
		bytes int64
	}
	events := map[string][]event{}
	for _, activity := range projected.Activities {
		r := byID[assignments[activity.ID]]
		if r.Metadata["workspaceStorageBudget"] == nil {
			continue
		}
		payload, _ := json.Marshal(r.Metadata["workspaceStorageBudget"])
		var b Snapshot
		_ = json.Unmarshal(payload, &b)
		var volume workflow.WorkspaceProjectionVolumes
		if activity.Metadata["workspaceProjectionVolumes"] == nil {
			return nil, missingIDs, fmt.Errorf("storage budget needs whole-workspace projection for %s", activity.ID)
		}
		payload, err := json.Marshal(activity.Metadata["workspaceProjectionVolumes"])
		if err != nil {
			return nil, missingIDs, err
		}
		if err = json.Unmarshal(payload, &volume); err != nil {
			return nil, missingIDs, err
		}
		// Both archive and received/extracted inputs may coexist with the merged
		// consumer and producer workspaces. Never account the same quota per node.
		workspaceBytes := int64(0)
		for _, bytes := range []int64{volume.PreExecutionBytes, volume.CreatedOutputBytes} {
			if bytes < 0 || bytes > math.MaxInt64-workspaceBytes {
				return nil, missingIDs, fmt.Errorf("storage reservation overflow")
			}
			workspaceBytes += bytes
		}
		bytesToReserve := []int64{workspaceBytes, volume.InputTotalBytes, volume.InputTotalBytes}
		if timings != nil {
			// Start preparation at the earliest predecessor completion (or zero
			// for roots), conservatively before the consumer command starts.
			begin, end, hasParent, hasConsumer := float64(0), timings[activity.ID].Finish, false, false
			for _, d := range edges {
				if d.ActivityID == activity.ID {
					parent, ok := timings[d.DependsOnActivityID]
					if !ok || parent.Finish > timings[activity.ID].Start {
						return nil, missingIDs, fmt.Errorf("noncausal storage schedule for %s", activity.ID)
					}
					if !hasParent || parent.Finish < begin {
						begin = parent.Finish
					}
					hasParent = true
				}
				if d.DependsOnActivityID == activity.ID {
					consumer, ok := timings[d.ActivityID]
					if !ok {
						return nil, missingIDs, fmt.Errorf("missing storage consumer %s", d.ActivityID)
					}
					end = math.Max(end, consumer.Start)
					hasConsumer = true
				}
			}
			if hasConsumer {
				events[b.StorageID] = append(events[b.StorageID], event{begin, workspaceBytes}, event{end, -workspaceBytes})
				bytesToReserve = bytesToReserve[1:]
			}
			// Leaves remain retained; no final-product pruning/export assumed.
		}
		for _, bytes := range bytesToReserve {
			if bytes < 0 || bytes > math.MaxInt64-reserved[b.StorageID] {
				return nil, missingIDs, fmt.Errorf("storage reservation overflow")
			}
			reserved[b.StorageID] += bytes
		}
	}
	for id, list := range events {
		sort.Slice(list, func(i, j int) bool {
			if list[i].at == list[j].at {
				return list[i].bytes > list[j].bytes
			}
			return list[i].at < list[j].at
		})
		current, peak := reserved[id], reserved[id]
		for _, e := range list {
			if e.bytes > 0 && e.bytes > math.MaxInt64-current {
				return nil, missingIDs, fmt.Errorf("storage reservation overflow")
			}
			current += e.bytes
			if current < 0 {
				return nil, missingIDs, fmt.Errorf("negative storage reservation")
			}
			if current > peak {
				peak = current
			}
		}
		reserved[id] = peak
	}
	ids := []string{}
	for id := range budgets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := []Reservation{}
	for _, id := range ids {
		b := budgets[id]
		result = append(result, Reservation{id, b.AvailableBytes, reserved[id], b.Source, b.ObservedAt})
	}
	for _, r := range result {
		if r.ReservedUpperBoundBytes > r.AvailableBytes {
			return result, missingIDs, &InsufficientQuota{Reservation: r}
		}
	}
	return result, missingIDs, nil
}
