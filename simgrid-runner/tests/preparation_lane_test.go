package tests

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNativePreparationStartsAfterPlannedLanePredecessor(t *testing.T) {
	image := os.Getenv("AKOFLOW_SIMGRID_TEST_IMAGE")
	if image == "" {
		t.Skip("set AKOFLOW_SIMGRID_TEST_IMAGE")
	}
	fixtures, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	for _, ordered := range []bool{false, true} {
		t.Run(map[bool]string{false: "independent", true: "ordered"}[ordered], func(t *testing.T) {
			var input map[string]any
			if e = json.Unmarshal([]byte(`{"materializationModelVersion":1,"preparationLaneBarrierVersion":1,"runId":"lane","planId":"lane","tasks":[{"id":"p","assignmentId":"p","resourceId":"cloud-a","flops":1000000000},{"id":"c","assignmentId":"c","resourceId":"cloud-b","flops":10000000000},{"id":"b","assignmentId":"b","resourceId":"cloud-b","flops":1000000000,"preparationSeconds":2}],"dependencies":[{"producerId":"p","consumerId":"b","bytes":100,"networkBytes":100,"strategy":"direct-runtime","stages":[{"name":"peer","sourceResourceId":"cloud-a","targetResourceId":"cloud-b","bytes":100,"effectiveBytesPerSecond":100}]}],"resourceLaneOrders":[]}`), &input); e != nil {
				t.Fatal(e)
			}
			if ordered {
				input["resourceLaneOrders"] = []map[string]string{{"predecessorId": "c", "successorId": "b"}}
			}
			dir := t.TempDir()
			b, e := json.Marshal(input)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, "input.json"), b, 0600); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command("docker", "run", "--rm", "-v", fixtures+":/fixtures:ro", "-v", dir+":/out", image, "/build/output/akoflow-simgrid-runner", "--platform", "/fixtures/materialization-platform.xml", "--input", "/out/input.json", "--output", "/out/result.json")
			if out, e := cmd.CombinedOutput(); e != nil {
				t.Fatalf("runner: %v %s", e, out)
			}
			b, e = os.ReadFile(filepath.Join(dir, "result.json"))
			if e != nil {
				t.Fatal(e)
			}
			var trace struct {
				Transfers []struct {
					StartedAt float64 `json:"startedAt"`
				}
				Tasks []struct {
					ActivityID            string `json:"activityId"`
					StartedAt, FinishedAt float64
				}
			}
			if e = json.Unmarshal(b, &trace); e != nil {
				t.Fatal(e)
			}
			want := 1.0
			if ordered {
				want = 10
			}
			if len(trace.Transfers) != 1 || math.Abs(trace.Transfers[0].StartedAt-want) > 1e-5 {
				t.Fatalf("preparation start=%+v want %v", trace.Transfers, want)
			}
			if ordered {
				for _, task := range trace.Tasks {
					if task.ActivityID == "b" && math.Abs(task.FinishedAt-14) > 1e-3 {
						t.Fatalf("blocked transfer was prefetched: %+v", task)
					}
				}
			}
		})
	}
}
