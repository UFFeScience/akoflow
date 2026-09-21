package planning

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestNormalizePairwiseSlowdownMatrix(t *testing.T) {
	workflow := domain.WorkflowVersion{ID: "workflow", Activities: []domain.Activity{{ID: "a"}, {ID: "b"}}}
	matrix, err := normalizeInterferenceMatrix(&domain.InterferenceMatrix{
		SchemaVersion: "2",
		Model:         "pairwise-slowdown",
		Entries: []domain.InterferenceEntry{{
			AffectedActivityID: "a", InterferingActivityID: "b", SlowdownFactor: 1.4,
		}},
	}, workflow)
	if err != nil {
		t.Fatalf("normalize slowdown matrix: %v", err)
	}
	if matrix.Aggregation != "maximum" {
		t.Fatalf("expected maximum aggregation, got %q", matrix.Aggregation)
	}
}

func TestNormalizePairwiseSlowdownRejectsSpeedup(t *testing.T) {
	workflow := domain.WorkflowVersion{ID: "workflow", Activities: []domain.Activity{{ID: "a"}, {ID: "b"}}}
	_, err := normalizeInterferenceMatrix(&domain.InterferenceMatrix{
		SchemaVersion: "2", Model: "pairwise-slowdown", Aggregation: "maximum",
		Entries: []domain.InterferenceEntry{{
			AffectedActivityID: "a", InterferingActivityID: "b", SlowdownFactor: 0.9,
		}},
	}, workflow)
	if err == nil {
		t.Fatal("expected slowdownFactor below one to be rejected")
	}
}

func TestNormalizePairwiseSlowdownFamilyRules(t *testing.T) {
	workflow := domain.WorkflowVersion{ID: "workflow", Activities: []domain.Activity{
		{ID: "a", ActivityTypeID: "type-a"},
		{ID: "b", ActivityTypeID: "type-b"},
	}}
	matrix, err := normalizeInterferenceMatrix(&domain.InterferenceMatrix{
		SchemaVersion: "2", Model: "pairwise-slowdown", Aggregation: "maximum",
		Rules: []domain.InterferenceRule{{
			AffectedActivityTypeID: "type-a", InterferingActivityTypeID: "type-b", SlowdownFactor: 1.4,
		}},
	}, workflow)
	if err != nil {
		t.Fatalf("normalize family slowdown rules: %v", err)
	}
	if len(matrix.Rules) != 1 || matrix.Rules[0].SlowdownFactor != 1.4 {
		t.Fatalf("unexpected normalized rules: %#v", matrix.Rules)
	}
}
