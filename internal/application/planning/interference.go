package planning

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const interferenceConfigurationKey = "interferenceMatrix"

func decodeInterferenceMatrix(value any) (*domain.InterferenceMatrix, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode interference matrix: %w", err)
	}
	var matrix domain.InterferenceMatrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		return nil, fmt.Errorf("decode interference matrix: %w", err)
	}
	return &matrix, nil
}

func normalizeInterferenceMatrix(matrix *domain.InterferenceMatrix, workflow domain.WorkflowVersion) (*domain.InterferenceMatrix, error) {
	if matrix == nil {
		return nil, nil
	}
	if matrix.SchemaVersion == "" {
		matrix.SchemaVersion = "1"
	}
	if matrix.SchemaVersion != "1" {
		return nil, fmt.Errorf("unsupported interference matrix schemaVersion %q", matrix.SchemaVersion)
	}
	if matrix.Model == "" {
		matrix.Model = "pairwise-cpu-priority"
	}
	if matrix.Model != "pairwise-cpu-priority" {
		return nil, fmt.Errorf("unsupported interference model %q", matrix.Model)
	}
	if matrix.Aggregation == "" {
		matrix.Aggregation = "minimum"
	}
	if matrix.Aggregation != "minimum" {
		return nil, fmt.Errorf("unsupported interference aggregation %q", matrix.Aggregation)
	}
	activities := make(map[string]bool, len(workflow.Activities))
	for _, activity := range workflow.Activities {
		activities[activity.ID] = true
	}
	seen := make(map[string]bool, len(matrix.Entries))
	for _, entry := range matrix.Entries {
		if !activities[entry.AffectedActivityID] || !activities[entry.InterferingActivityID] {
			return nil, fmt.Errorf("interference pair %q -> %q references an activity outside workflow %q", entry.InterferingActivityID, entry.AffectedActivityID, workflow.ID)
		}
		if entry.AffectedActivityID == entry.InterferingActivityID {
			return nil, fmt.Errorf("interference pair cannot reference the same activity %q", entry.AffectedActivityID)
		}
		if entry.PriorityWeight <= 0 {
			return nil, fmt.Errorf("interference pair %q -> %q must have priorityWeight > 0", entry.InterferingActivityID, entry.AffectedActivityID)
		}
		key := entry.AffectedActivityID + "\x00" + entry.InterferingActivityID
		if seen[key] {
			return nil, fmt.Errorf("duplicate interference pair %q -> %q", entry.InterferingActivityID, entry.AffectedActivityID)
		}
		seen[key] = true
	}
	sort.Slice(matrix.Entries, func(i, j int) bool {
		left := matrix.Entries[i].AffectedActivityID + "\x00" + matrix.Entries[i].InterferingActivityID
		right := matrix.Entries[j].AffectedActivityID + "\x00" + matrix.Entries[j].InterferingActivityID
		return left < right
	})
	return matrix, nil
}
