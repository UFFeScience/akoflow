package main

import (
	"reflect"
	"testing"
)

func TestMatchedInputPreservesScientificAndPreparationDifferences(t *testing.T) {
	peer := obj{
		"Run":                               obj{"id": "heft", "mode": "real", "seed": 1},
		"Plan":                              obj{"id": "heft-plan"},
		"Resources":                         []obj{{"id": "hpc", "metadata": obj{"workspaceStorageQuotaOverride": obj{"runId": "heft"}}}},
		"NetworkTopology":                   obj{"bandwidth": 10},
		"PreparationRequirementsByActivity": obj{"task": "frozen-requirement"},
	}
	baseline, err := matchedScientificInput(peer)
	if err != nil {
		t.Fatal(err)
	}
	followup := clone(peer)
	followup["Run"].(map[string]any)["id"] = "prism"
	followup["Plan"] = obj{"id": "prism-plan"}
	followup["Resources"].([]any)[0].(map[string]any)["metadata"].(map[string]any)["workspaceStorageQuotaOverride"] = obj{"runId": "prism"}
	actual, err := matchedScientificInput(followup)
	if err != nil || !reflect.DeepEqual(baseline, actual) {
		t.Fatal("run-scoped identity incorrectly changes scientific inputs", err)
	}
	for _, field := range []string{"NetworkTopology", "PreparationRequirementsByActivity"} {
		changed := clone(followup)
		changed[field] = obj{"changed": true}
		actual, err := matchedScientificInput(changed)
		if err != nil || reflect.DeepEqual(baseline, actual) {
			t.Fatalf("scientific drift in %s was discarded", field)
		}
	}
	followup["Run"].(map[string]any)["seed"] = 2
	if _, err := matchedScientificInput(followup); err == nil {
		t.Fatal("different seed accepted")
	}
}
