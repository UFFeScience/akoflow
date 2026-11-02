//go:build ignore

// Run from the repository root: go run experiments/prism-medium-strict-20261031/simulation_fidelity.go
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type task struct {
	StartedAt  float64 `json:"startedAt"`
	FinishedAt float64 `json:"finishedAt"`
	Metadata   struct {
		Publication float64 `json:"workspacePublicationSeconds"`
	} `json:"metadata"`
}

func main() {
	const realSeconds = 5212.838178634644
	const root = "outputs/simulation-fidelity-20261101/"
	rows := []map[string]any{}
	for _, name := range []string{"behavior-only", "hpc-calibrated", "all-calibrated"} {
		path := root + name + "/result.json"
		b, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		var trace struct {
			Tasks    []task `json:"tasks"`
			Executed struct {
				Makespan float64 `json:"makespanSeconds"`
			} `json:"executed"`
		}
		if err := json.Unmarshal(b, &trace); err != nil {
			panic(err)
		}
		if len(trace.Tasks) != 58 {
			panic("expected 58 simulated activities")
		}
		first, last := math.Inf(1), 0.0
		for _, t := range trace.Tasks {
			first = math.Min(first, t.StartedAt)
			// Native finishedAt includes publication; real activity finishedAt
			// precedes publication. Keep publication causal for all successors.
			last = math.Max(last, t.FinishedAt-t.Metadata.Publication)
		}
		span := last - first
		rows = append(rows, map[string]any{
			"case": name, "source": path,
			"sourceSHA256":                     fmt.Sprintf("%x", sha256.Sum256(b)),
			"engineEndSeconds":                 trace.Executed.Makespan,
			"activitySpanSeconds":              span,
			"activitySpanAbsoluteErrorSeconds": math.Abs(span - realSeconds),
			"activitySpanAbsoluteErrorPercent": 100 * math.Abs(span-realSeconds) / realSeconds,
			"sameTraceCalibration":             name != "behavior-only",
		})
	}
	out := map[string]any{
		"realActivitySpanSeconds":   realSeconds,
		"originalSimulationSeconds": 3007.1380910688645,
		"independentValidation":     false,
		"cases":                     rows,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
