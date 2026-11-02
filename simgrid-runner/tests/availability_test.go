package tests

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNativeRunnerInitialBootDoesNotRepeatOrDelayLateArrivals(t *testing.T) {
	image := os.Getenv("AKOFLOW_SIMGRID_TEST_IMAGE")
	if image == "" {
		t.Skip("set AKOFLOW_SIMGRID_TEST_IMAGE to the built runner image")
	}
	fixtures, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input := `{"runId":"boot","planId":"boot","tasks":[{"id":"a","assignmentId":"a","resourceId":"edge","flops":2000000000,"resourceAvailableAtSeconds":12,"overheadSeconds":2},{"id":"b","assignmentId":"b","resourceId":"edge","flops":2000000000,"resourceAvailableAtSeconds":12,"overheadSeconds":2},{"id":"c","assignmentId":"c","resourceId":"cloud","flops":120000000000},{"id":"d","assignmentId":"d","resourceId":"edge","flops":2000000000,"resourceAvailableAtSeconds":12,"overheadSeconds":2}],"dependencies":[{"producerId":"a","consumerId":"b","bytes":0},{"producerId":"c","consumerId":"d","bytes":0}],"resourceLaneOrders":[{"predecessorId":"a","successorId":"b"},{"predecessorId":"b","successorId":"d"}]}`
	if err = os.WriteFile(filepath.Join(dir, "input.json"), []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("docker", "run", "--rm", "-v", fixtures+":/fixtures:ro", "-v", dir+":/out", image, "/build/output/akoflow-simgrid-runner", "--platform", "/fixtures/platform.xml", "--input", "/out/input.json", "--output", "/out/result.json")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("runner: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var trace struct {
		Tasks []struct {
			ActivityID            string `json:"activityId"`
			StartedAt, FinishedAt float64
		}
	}
	if err = json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	expected := map[string][2]float64{"a": {12, 16}, "b": {16, 20}, "c": {0, 30}, "d": {30, 34}}
	if len(trace.Tasks) != 4 {
		t.Fatal(string(data))
	}
	for _, task := range trace.Tasks {
		if task.ActivityID == "d" {
			if task.StartedAt < 30 || task.StartedAt >= 32 || math.Abs(task.FinishedAt-task.StartedAt-4) > 1e-9 {
				t.Fatalf("late arrival paid boot again: %+v", task)
			}
			continue
		}
		want, ok := expected[task.ActivityID]
		if !ok || task.StartedAt != want[0] || task.FinishedAt != want[1] {
			t.Fatalf("task %+v; want %v", task, want)
		}
	}
}
