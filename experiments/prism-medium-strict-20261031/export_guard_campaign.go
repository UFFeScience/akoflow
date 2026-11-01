//go:build ignore

// Export only complete, audited frozen Figure 8 deployments.
package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type obj = map[string]any

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string, v any) string {
	b, e := os.ReadFile(p)
	must(e)
	must(json.Unmarshal(b, v))
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func write(p string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	must(e)
	must(os.WriteFile(p, b, 0600))
}
func number(m map[string]any, k string) float64 { v, _ := m[k].(float64); return v }
func planSpan(p domain.SchedulePlan) float64 {
	first, last := math.Inf(1), 0.0
	if len(p.Assignments) != 58 {
		panic("incomplete plan")
	}
	for _, a := range p.Assignments {
		// The scheduler reports compute start after fixed boot/container overhead.
		first = math.Min(first, a.PredictedStartAt-number(a.Metadata, "bootOverheadSeconds")-number(a.Metadata, "containerOverheadSeconds"))
		last = math.Max(last, a.PredictedFinishAt-number(a.Metadata, "workspacePublicationSeconds"))
	}
	return last - first
}
func span(tasks []domain.TaskExecution, native bool) float64 {
	first, last := math.Inf(1), 0.0
	if len(tasks) != 58 {
		panic("58 activities required")
	}
	seen := map[string]bool{}
	for _, a := range tasks {
		if a.Status != domain.TaskCompleted || seen[a.ActivityID] || a.FinishedAt < a.StartedAt {
			panic("incomplete or duplicate activity")
		}
		seen[a.ActivityID] = true
		first = math.Min(first, a.StartedAt)
		finish := a.FinishedAt
		if native {
			finish -= number(a.Metadata, "workspacePublicationSeconds")
		}
		last = math.Max(last, finish)
	}
	return last - first
}

type realTrace struct {
	Run        domain.ExecutionRun
	Activities []domain.TaskExecution
	Handles    []domain.ActivityHandle
}
type manifestRow struct{ Algorithm, Environment, Group, InputSHA, PlanSHA, NativeSHA, PlanningSHA string }
type deployment struct{ Directory, RunID, Submitted string }

func main() {
	if len(os.Args) != 3 {
		panic("usage: export_guard_campaign.go ROOT DEPLOYMENTS_JSON")
	}
	root := os.Args[1]
	var manifest []manifestRow
	read(filepath.Join(root, "frozen/manifest.json"), &manifest)
	if len(manifest) != 6 {
		panic("six frozen cases required")
	}
	var sources map[string]deployment
	read(os.Args[2], &sources)
	rows := []obj{}
	for _, m := range manifest {
		capsule := filepath.Join(root, "frozen", m.Algorithm+"-"+m.Environment)
		var plan domain.SchedulePlan
		if read(filepath.Join(capsule, "plan.json"), &plan) != m.PlanSHA {
			panic("plan changed after freeze")
		}
		var native domain.ExecutionTrace
		if read(filepath.Join(capsule, "native-result.json"), &native) != m.NativeSHA {
			panic("simulation changed after freeze")
		}
		var raw any
		if read(filepath.Join(capsule, "request.json"), &raw) != m.InputSHA || read(filepath.Join(capsule, "planning-request.json"), &raw) != m.PlanningSHA {
			panic("frozen inputs changed")
		}
		source, ok := sources[m.Group]
		if !ok {
			panic("missing real deployment " + m.Group)
		}
		var real realTrace
		realPath := filepath.Join(source.Directory, "strict-live-monitor.json")
		historical := m.Group == "prism-time-hpc"
		if historical {
			realPath = filepath.Join(source.Directory, "real-results.json")
		}
		realSHA := read(realPath, &real)
		if string(real.Run.Status) != "completed" || real.Run.ID != source.RunID {
			panic("real run identity or completion mismatch")
		}
		r := span(real.Activities, false)
		if math.Abs(r-real.Run.MakespanSeconds) > 0.01 {
			panic("real duration boundary mismatch")
		}
		var submitted struct{ Plan domain.SchedulePlan }
		submittedSHA := read(source.Submitted, &submitted)
		if historical {
			var proof struct {
				Equivalent                                 bool `json:"equivalentUpToIdenticalCoreLabels"`
				ChangedResource, ChangedSlot, ChangedOrder int
			}
			read(filepath.Join(root, "historical-prism-hpc-equivalence.json"), &proof)
			if !proof.Equivalent || proof.ChangedResource != 0 || proof.ChangedSlot != 0 || proof.ChangedOrder != 0 {
				panic("historical equivalence not proved")
			}
		} else {
			var representative domain.SchedulePlan
			read(filepath.Join(root, "frozen", m.Group, "plan.json"), &representative)
			if !reflect.DeepEqual(submitted.Plan, representative) || real.Run.SchedulePlanID != representative.ID {
				panic("executed plan differs from frozen representative")
			}
			var audit struct {
				AuditVersion int
				Complete     bool
				Violations   []string
				RunID        string
			}
			read(filepath.Join(source.Directory, "strict-order-audit.json"), &audit)
			if audit.AuditVersion < 2 || !audit.Complete || len(audit.Violations) != 0 || audit.RunID != source.RunID {
				panic("complete zero-violation order audit required")
			}
			var products struct {
				Complete bool
				RunID    string
			}
			read(filepath.Join(source.Directory, "final-products-audit.json"), &products)
			if !products.Complete || products.RunID != source.RunID {
				panic("verified final products required")
			}
		}
		p, s := planSpan(plan), span(native.Tasks, true)
		var simulatedBytes int64
		for _, task := range native.Tasks {
			simulatedBytes += task.TransferBytes
		}
		if p <= 0 || s <= 0 || r <= 0 {
			panic("invalid makespan")
		}
		lo, hi := math.Min(p, math.Min(s, r)), math.Max(p, math.Max(s, r))
		version := plan.AlgorithmVersion
		wall := 0.0
		if real.Run.StartedAt != nil && real.Run.FinishedAt != nil {
			wall = real.Run.FinishedAt.Sub(*real.Run.StartedAt).Seconds()
		}
		rows = append(rows, obj{
			"environment": m.Environment, "algorithm": m.Algorithm, "algorithm_version": version, "candidate_origin": plan.Metadata["candidateOrigin"],
			"planned_makespan_seconds": p, "simulated_makespan_seconds": s, "real_makespan_seconds": r,
			"planned_original_end_clock_seconds": plan.Predicted.MakespanSeconds, "simulated_original_end_clock_seconds": native.Executed.MakespanSeconds,
			"real_run_wall_seconds": wall, "planned_absolute_error_percent": 100 * math.Abs(p-r) / r, "simulation_absolute_error_percent": 100 * math.Abs(s-r) / r,
			"simulation_signed_error_percent": 100 * (s - r) / r, "range_seconds": hi - lo, "range_percent_of_planned": 100 * (hi - lo) / p,
			"planned_cost_usd": plan.Predicted.Cost, "simulated_cost_usd": native.Executed.Cost, "real_cost_usd": real.Run.Cost,
			"simulated_transfer_bytes": simulatedBytes, "real_transfer_bytes": real.Run.TransferredBytes,
			"simulated_transfer_seconds": native.Executed.TransferSeconds, "real_transfer_seconds": real.Run.Breakdown.TransferSeconds,
			"cost_basis":  "scheduler/execution estimates using frozen resource prices; execution cost includes observed allocation idle/disk costs; not a cloud invoice",
			"real_run_id": real.Run.ID, "plan_id": plan.ID, "simulation_run_id": native.RunID, "shared_real_group": m.Group, "historical_real_protocol": historical,
			"real_status": "completed", "boundary": "first activity start (before launch overhead) to final process completion; excludes initial preparation/admission and final workspace publication",
			"cohort":            "frozen calibrated input; three fresh distinct deployments and one equivalent historical HPC deployment",
			"real_trace_source": realPath, "real_trace_sha256": realSHA, "submitted_request_source": source.Submitted, "submitted_request_sha256": submittedSHA,
			"input_sha256": m.InputSHA, "plan_sha256": m.PlanSHA, "planning_sha256": m.PlanningSHA, "native_trace_sha256": m.NativeSHA,
			"real_plan_audit": filepath.Join(source.Directory, "strict-order-audit.json"), "exported_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
	write(filepath.Join(root, "figure8-observations.json"), rows)
	f, e := os.Create(filepath.Join(root, "figure8-observations.csv"))
	must(e)
	defer f.Close()
	w := csv.NewWriter(f)
	columns := []string{"environment", "algorithm", "algorithm_version", "candidate_origin", "planned_makespan_seconds", "simulated_makespan_seconds", "real_makespan_seconds", "planned_absolute_error_percent", "simulation_absolute_error_percent", "simulation_signed_error_percent", "range_seconds", "range_percent_of_planned", "planned_cost_usd", "simulated_cost_usd", "real_cost_usd", "real_run_id", "shared_real_group", "historical_real_protocol", "plan_id", "simulation_run_id", "planned_original_end_clock_seconds", "simulated_original_end_clock_seconds", "real_run_wall_seconds", "boundary", "real_trace_source", "real_trace_sha256", "submitted_request_sha256", "input_sha256", "plan_sha256", "planning_sha256", "native_trace_sha256"}
	columns = append(columns, "simulated_transfer_bytes", "real_transfer_bytes", "simulated_transfer_seconds", "real_transfer_seconds", "cost_basis")
	must(w.Write(columns))
	for _, r := range rows {
		line := []string{}
		for _, k := range columns {
			v := r[k]
			if n, ok := v.(float64); ok {
				line = append(line, strconv.FormatFloat(n, 'f', 9, 64))
			} else if v == nil {
				line = append(line, "")
			} else {
				line = append(line, fmt.Sprint(v))
			}
		}
		must(w.Write(line))
	}
	w.Flush()
	must(w.Error())
	fmt.Println("Exported six verified Figure 8 observations")
}
