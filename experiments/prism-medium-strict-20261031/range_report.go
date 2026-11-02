//go:build ignore

// Export a completed fresh execution and the six Figure 8 observations.
package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type object = map[string]any

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func read(path string, value any) string {
	b, err := os.ReadFile(path)
	must(err)
	must(json.Unmarshal(b, value))
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func save(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, b, 0600))
}
func number(v any) float64 { n, _ := v.(float64); return n }
func str(v any) string     { s, _ := v.(string); return s }

type native struct {
	Executed struct{ MakespanSeconds float64 }
	Tasks    []struct {
		StartedAt, FinishedAt float64
		Metadata              object
	}
}

func nativeSpan(t native) float64 {
	if len(t.Tasks) != 58 {
		panic("58 native activities required")
	}
	first, last := math.Inf(1), 0.0
	for _, a := range t.Tasks {
		first = math.Min(first, a.StartedAt)
		last = math.Max(last, a.FinishedAt-number(a.Metadata["workspacePublicationSeconds"]))
	}
	return last - first
}
func planSpan(p domain.SchedulePlan) float64 {
	if len(p.Assignments) != 58 {
		panic("58 plan assignments required")
	}
	first, last := math.Inf(1), 0.0
	for _, a := range p.Assignments {
		// PRISM predictedStartAt is compute start, after fixed launch overhead;
		// real activity.startedAt and native startedAt precede this overhead.
		start := a.PredictedStartAt - number(a.Metadata["bootOverheadSeconds"]) - number(a.Metadata["containerOverheadSeconds"])
		finish := a.PredictedFinishAt - number(a.Metadata["workspacePublicationSeconds"])
		first = math.Min(first, start)
		last = math.Max(last, finish)
	}
	return last - first
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: range_report.go COMPLETED_CAMPAIGN_DIRECTORY")
	}
	dir := os.Args[1]
	var audit struct {
		Complete   bool
		Violations []string
	}
	read(filepath.Join(dir, "strict-order-audit.json"), &audit)
	if !audit.Complete || len(audit.Violations) != 0 {
		panic("complete zero-violation audit required")
	}
	var real struct {
		Run struct {
			ID, Status, SchedulePlanID string
			MakespanSeconds            float64
		}
		Activities []domain.TaskExecution
		Handles    []domain.ActivityHandle
	}
	realSHA := read(filepath.Join(dir, "strict-live-monitor.json"), &real)
	if real.Run.Status != "completed" || len(real.Activities) != 58 {
		panic("completed 58 activity real observation required")
	}
	var hpcPlan, mediumPlan domain.SchedulePlan
	read("outputs/prism-time-recalibrated-final-20261101-hpc/selected-plan.json", &hpcPlan)
	read(filepath.Join(dir, "medium-plan.json"), &mediumPlan)
	if real.Run.SchedulePlanID != mediumPlan.ID || len(real.Handles) != 58 {
		panic("exact frozen plan and 58 single-attempt handles required")
	}
	for _, a := range real.Activities {
		if a.Attempt != 1 {
			panic("recovered activity cannot be silently exported as fresh single attempt")
		}
	}
	for _, h := range real.Handles {
		if h.Status != domain.HandleCompleted || h.ExitCode == nil || *h.ExitCode != 0 {
			panic("completed successful handles required")
		}
	}
	var hpcNative, mediumNative native
	read("outputs/prism-time-recalibrated-final-20261101-hpc/native-verification/verified-result.json", &hpcNative)
	read("outputs/prism-time-recalibrated-final-20261101-medium/native-verification/verified-result.json", &mediumNative)
	var identity object
	read("experiments/prism-medium-strict-20261031/hpc-recalibrated-plan-audit.json", &identity)
	if identity["equivalentUpToIdenticalCoreLabels"] != true {
		panic("HPC historical reference no longer equivalent")
	}
	var rows []object
	read(filepath.Join(dir, "figure-data-before-run.json"), &rows)
	if len(rows) != 6 {
		panic("six figure rows required")
	}
	for _, r := range rows {
		if r["algorithm"] == "prism-time" {
			p, t := hpcPlan, hpcNative
			if r["environment"] == "medium" {
				p, t = mediumPlan, mediumNative
				r["real_makespan_seconds"] = real.Run.MakespanSeconds
				r["real_run_id"] = real.Run.ID
				r["cohort"] = "fresh-real-validation"
				r["real_trace_sha256"] = realSHA
				r["real_status"] = "completed on exact frozen plan"
				r["simulation_run_id"] = strings.TrimSuffix(real.Run.ID, "-real") + "-simulation"
				r["real_plan_audit"] = filepath.Join(dir, "strict-order-audit.json")
			} else {
				r["cohort"] = "recalibrated-model-historical-equivalent-placement"
				r["real_status"] = "historical equivalent logical-core placement"
				r["real_plan_audit"] = "experiments/prism-medium-strict-20261031/hpc-recalibrated-plan-audit.json"
			}
			caseDir := "outputs/prism-time-recalibrated-final-20261101-" + str(r["environment"])
			r["planning_source"] = filepath.Join(caseDir, "planning-request.json")
			r["simulation_source"] = filepath.Join(caseDir, "native-verification/verified-result.json")
			r["planned_engine_end_seconds"] = p.Predicted.MakespanSeconds
			r["simulated_engine_end_seconds"] = t.Executed.MakespanSeconds
			r["planned_makespan_seconds"] = planSpan(p)
			r["simulated_makespan_seconds"] = nativeSpan(t)
			r["plan_id"] = p.ID
			r["boundary"] = "first activity start to last activity finish; exclude final publication; retain causal intermediate publication"
			r["calibration_fit"] = false
			r["simulation_boundary"] = r["boundary"]
			delete(r, "simulation_engine_end_seconds")
		}
		p, s, z := number(r["planned_makespan_seconds"]), number(r["simulated_makespan_seconds"]), number(r["real_makespan_seconds"])
		rangeSeconds := math.Max(p, math.Max(s, z)) - math.Min(p, math.Min(s, z))
		r["range_seconds"] = rangeSeconds
		r["range_percent_of_planned"] = 100 * rangeSeconds / p
		r["simulation_absolute_error_percent"] = 100 * math.Abs(s-z) / z
	}
	save(filepath.Join(dir, "figure8-range-data.json"), rows)
	f, err := os.Create(filepath.Join(dir, "figure8-range-data.csv"))
	must(err)
	defer f.Close()
	w := csv.NewWriter(f)
	header := []string{"environment", "algorithm", "planned_makespan_seconds", "simulated_makespan_seconds", "real_makespan_seconds", "range_seconds", "range_percent_of_planned", "simulation_absolute_error_percent", "planned_engine_end_seconds", "simulated_engine_end_seconds", "real_run_id", "cohort"}
	must(w.Write(header))
	for _, r := range rows {
		line := make([]string, len(header))
		for i, k := range header {
			if _, ok := r[k].(float64); ok {
				line[i] = fmt.Sprintf("%.12f", number(r[k]))
			} else {
				line[i] = str(r[k])
			}
		}
		must(w.Write(line))
	}
	w.Flush()
	must(w.Error())
	fmt.Println("Exported six observations with common activity-span boundaries and original model end clocks")
}
