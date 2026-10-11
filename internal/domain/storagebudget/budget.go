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

// Check projects runtime-specific manifests and reserves all workspaces plus
// packed/received input staging until run end. This deliberately does not grant
// early-release savings unless the execution contract proves those releases.
// Shared logical nodes count against one identical effective quota snapshot.
// Missing budgets are returned explicitly; legacy workflows remain compatible.
func Check(version workflow.WorkflowVersion, resources []resource.Resource, assignments map[string]string) ([]Reservation, []string, error) {
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
	projected, err := workflow.ProjectWorkspaceDependenciesForResourceTypes(version, types)
	if err != nil {
		return nil, missingIDs, err
	}
	reserved := map[string]int64{}
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
		for _, bytes := range []int64{volume.PreExecutionBytes, volume.CreatedOutputBytes, volume.InputTotalBytes, volume.InputTotalBytes} {
			if bytes < 0 || bytes > math.MaxInt64-reserved[b.StorageID] {
				return nil, missingIDs, fmt.Errorf("storage reservation overflow")
			}
			reserved[b.StorageID] += bytes
		}
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
			return result, missingIDs, fmt.Errorf("workspace storage quota %s insufficient: available %d bytes, conservative reservation %d bytes; verified cleanup/export or more quota required", r.StorageID, r.AvailableBytes, r.ReservedUpperBoundBytes)
		}
	}
	return result, missingIDs, nil
}
