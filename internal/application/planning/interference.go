package planning

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const (
	interferenceConfigurationKey          = "interferenceMatrix"
	interferenceKnowledgeConfigurationKey = "interferenceKnowledgeMatrix"
)

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
	if matrix.SchemaVersion != "1" && matrix.SchemaVersion != "2" && matrix.SchemaVersion != "3" && matrix.SchemaVersion != "4" {
		return nil, fmt.Errorf("unsupported interference matrix schemaVersion %q", matrix.SchemaVersion)
	}
	if matrix.Model == "" {
		matrix.Model = "pairwise-cpu-priority"
	}
	if matrix.Model != "pairwise-cpu-priority" && matrix.Model != "pairwise-slowdown" {
		return nil, fmt.Errorf("unsupported interference model %q", matrix.Model)
	}
	if matrix.Aggregation == "" {
		if matrix.Model == "pairwise-slowdown" {
			matrix.Aggregation = "maximum"
		} else {
			matrix.Aggregation = "minimum"
		}
	}
	if (matrix.Model == "pairwise-cpu-priority" && matrix.Aggregation != "minimum") ||
		(matrix.Model == "pairwise-slowdown" && matrix.Aggregation != "maximum" && matrix.Aggregation != "additive-excess") {
		return nil, fmt.Errorf("unsupported interference aggregation %q", matrix.Aggregation)
	}
	activities := make(map[string]bool, len(workflow.Activities))
	activityTypes := make(map[string]bool)
	for _, activity := range workflow.Activities {
		activities[activity.ID] = true
		activityTypes[activity.ActivityTypeID] = true
	}
	seen := make(map[string]bool, len(matrix.Entries))
	for _, entry := range matrix.Entries {
		if !activities[entry.AffectedActivityID] || !activities[entry.InterferingActivityID] {
			return nil, fmt.Errorf("interference pair %q -> %q references an activity outside workflow %q", entry.InterferingActivityID, entry.AffectedActivityID, workflow.ID)
		}
		if entry.AffectedActivityID == entry.InterferingActivityID {
			return nil, fmt.Errorf("interference pair cannot reference the same activity %q", entry.AffectedActivityID)
		}
		if matrix.Model == "pairwise-cpu-priority" && entry.PriorityWeight <= 0 {
			return nil, fmt.Errorf("interference pair %q -> %q must have priorityWeight > 0", entry.InterferingActivityID, entry.AffectedActivityID)
		}
		if matrix.Model == "pairwise-slowdown" && entry.SlowdownFactor < 1 {
			return nil, fmt.Errorf("interference pair %q -> %q must have slowdownFactor >= 1", entry.InterferingActivityID, entry.AffectedActivityID)
		}
		key := entry.AffectedActivityID + "\x00" + entry.InterferingActivityID
		if seen[key] {
			return nil, fmt.Errorf("duplicate interference pair %q -> %q", entry.InterferingActivityID, entry.AffectedActivityID)
		}
		seen[key] = true
	}
	ruleSeen := make(map[string]bool, len(matrix.Rules))
	for _, rule := range matrix.Rules {
		if matrix.Model != "pairwise-slowdown" {
			return nil, fmt.Errorf("activity-family interference rules require pairwise-slowdown")
		}
		if !activityTypes[rule.AffectedActivityTypeID] || !activityTypes[rule.InterferingActivityTypeID] {
			return nil, fmt.Errorf("interference rule %q -> %q references an activity type outside workflow %q", rule.InterferingActivityTypeID, rule.AffectedActivityTypeID, workflow.ID)
		}
		if rule.SlowdownFactor < 1 {
			return nil, fmt.Errorf("interference rule %q -> %q must have slowdownFactor >= 1", rule.InterferingActivityTypeID, rule.AffectedActivityTypeID)
		}
		key := rule.AffectedActivityTypeID + "\x00" + rule.InterferingActivityTypeID
		if ruleSeen[key] {
			return nil, fmt.Errorf("duplicate interference rule %q -> %q", rule.InterferingActivityTypeID, rule.AffectedActivityTypeID)
		}
		ruleSeen[key] = true
	}
	if err := normalizeInterferenceGroups(matrix, activities, workflow.ID); err != nil {
		return nil, err
	}
	sortInterferenceMatrix(matrix)
	return matrix, nil
}

func sortInterferenceMatrix(matrix *domain.InterferenceMatrix) {
	sort.Slice(matrix.Entries, func(i, j int) bool {
		left := matrix.Entries[i].AffectedActivityID + "\x00" + matrix.Entries[i].InterferingActivityID
		right := matrix.Entries[j].AffectedActivityID + "\x00" + matrix.Entries[j].InterferingActivityID
		return left < right
	})
	sort.Slice(matrix.Rules, func(i, j int) bool {
		left := matrix.Rules[i].AffectedActivityTypeID + "\x00" + matrix.Rules[i].InterferingActivityTypeID
		right := matrix.Rules[j].AffectedActivityTypeID + "\x00" + matrix.Rules[j].InterferingActivityTypeID
		return left < right
	})
	sort.Slice(matrix.Groups, func(i, j int) bool { return matrix.Groups[i].ID < matrix.Groups[j].ID })
}

func normalizeInterferenceGroups(matrix *domain.InterferenceMatrix, activities map[string]bool, workflowID string) error {
	groupSeen := make(map[string]bool, len(matrix.Groups))
	for groupIndex := range matrix.Groups {
		group := &matrix.Groups[groupIndex]
		if matrix.Model != "pairwise-slowdown" {
			return fmt.Errorf("activity interference groups require pairwise-slowdown")
		}
		if group.ID == "" {
			return fmt.Errorf("interference group id is required")
		}
		if groupSeen[group.ID] {
			return fmt.Errorf("duplicate interference group %q", group.ID)
		}
		groupSeen[group.ID] = true
		if group.SlowdownFactor < 1 {
			return fmt.Errorf("interference group %q must have slowdownFactor >= 1", group.ID)
		}
		memberSeen := make(map[string]bool, len(group.ActivityIDs))
		for _, activityID := range group.ActivityIDs {
			if !activities[activityID] {
				return fmt.Errorf("interference group %q references activity %q outside workflow %q", group.ID, activityID, workflowID)
			}
			if memberSeen[activityID] {
				return fmt.Errorf("interference group %q contains duplicate activity %q", group.ID, activityID)
			}
			memberSeen[activityID] = true
		}
		sort.Strings(group.ActivityIDs)
	}
	return nil
}
