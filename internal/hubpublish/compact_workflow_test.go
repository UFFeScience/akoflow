package hubpublish

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLargeSharedBuildExperimentRemainsPortable(t *testing.T) {
	const activityCount = 514
	archive := strings.Repeat("a", 6500)
	inspection := strings.Repeat("i", 11000)
	activities := make([]any, 0, activityCount)
	for i := 0; i < activityCount; i++ {
		name := fmt.Sprintf("batch-%03d", i)
		source := map[string]any{
			"type": "build",
			"origin": map[string]any{"build": map[string]any{
				"contextArchive": archive, "contextDigest": "sha256:context",
			}},
			"inspection": map[string]any{"schemaVersion": 1, "data": map[string]any{"marker": inspection}},
		}
		activities = append(activities, map[string]any{
			"id": name, "name": name,
			"command": map[string]any{"executable": map[string]any{"source": source}},
		})
	}
	workflow := map[string]any{"id": "workflow", "name": "Workflow", "version": map[string]any{
		"id": "version", "activities": activities,
	}}
	context := map[string]any{"workflow": workflow, "plan": map[string]any{"id": "plan", "workflowVersionId": "version"}}
	execution := map[string]any{"run": map[string]any{"id": "run", "schedulePlanId": "plan", "workflowVersionId": "version"}}
	capture, err := buildExperiment(context, execution, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := marshalExperiment(capture.bundle); err != nil {
		t.Fatal(err)
	}
	var portable struct {
		Spec struct {
			Activities []struct {
				Command struct {
					Executable struct {
						Source map[string]any
					}
				}
			}
		}
	}
	if err := yaml.Unmarshal([]byte(capture.yaml), &portable); err != nil {
		t.Fatal(err)
	}
	if len(portable.Spec.Activities) != activityCount {
		t.Fatalf("expected %d portable activities, got %d", activityCount, len(portable.Spec.Activities))
	}
	for i, activity := range portable.Spec.Activities {
		source := activity.Command.Executable.Source
		encoded, err := json.Marshal(source)
		if err != nil || !strings.Contains(string(encoded), archive) || !strings.Contains(string(encoded), inspection) {
			t.Fatalf("portable activity %d lost its build context or inspection", i)
		}
	}
	for _, file := range capture.bundle.Files {
		if file.Path == "workflow.json" || file.Path == "plans.json" {
			if strings.Count(file.Content, archive) != 1 || strings.Count(file.Content, inspection) != 1 ||
				strings.Count(file.Content, "sha256:context") != activityCount {
				t.Fatalf("%s lost provenance or still repeats its payloads", file.Path)
			}
		}
	}
}

func TestLargeCaptureDropsOnlyIdenticalAttemptProjection(t *testing.T) {
	activity := map[string]any{"activityId": "block", "attempt": 1, "status": "completed"}
	execution := map[string]any{"activities": []any{activity}, "activityAttempts": []any{activity}}
	if !compactIdenticalActivityAttempts(execution) || execution["activityAttempts"] != nil {
		t.Fatal("duplicate attempt projection remained")
	}
	execution["activityAttempts"] = []any{
		map[string]any{"activityId": "block", "attempt": 1, "status": "failed"},
		activity,
	}
	if compactIdenticalActivityAttempts(execution) || execution["activityAttempts"] == nil {
		t.Fatal("distinct attempt history was lost")
	}
}
