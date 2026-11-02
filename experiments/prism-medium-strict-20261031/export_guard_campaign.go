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
	"strings"
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
	Run                      domain.ExecutionRun
	Activities               []domain.TaskExecution
	Handles                  []domain.ActivityHandle
	DataTransfers            []domain.DataTransfer
	ArtifactMaterializations []domain.ArtifactMaterialization
}
type manifestRow struct{ Algorithm, Environment, Group, InputSHA, PlanSHA, NativeSHA, PlanningSHA string }
type deployment struct {
	Directory, RunID, Submitted string
	Historical                  bool
}
type replay struct{ Result, Submitted string }

func scientificRequest(path string) map[string]any {
	var request map[string]any
	read(path, &request)
	delete(request, "Run")
	for _, value := range request["Resources"].([]any) {
		resource := value.(map[string]any)
		if metadata, ok := resource["metadata"].(map[string]any); ok {
			delete(metadata, "workspaceStorageQuotaOverride")
		}
	}
	return request
}

func main() {
	if len(os.Args) < 3 || len(os.Args) > 4 || (len(os.Args) == 4 && os.Args[3] != "--require-independent") {
		panic("usage: export_guard_campaign.go ROOT DEPLOYMENTS_JSON [--require-independent]")
	}
	root := os.Args[1]
	var manifest []manifestRow
	read(filepath.Join(root, "frozen/manifest.json"), &manifest)
	if len(manifest) != 6 {
		panic("six frozen cases required")
	}
	var sources map[string]deployment
	read(os.Args[2], &sources)
	if len(os.Args) == 4 {
		seen := map[string]bool{}
		for _, m := range manifest {
			key := m.Algorithm + "-" + m.Environment
			source, exists := sources[key]
			if !exists || source.RunID == "" || source.Historical || seen[source.RunID] {
				panic("independent real run required for " + key)
			}
			seen[source.RunID] = true
		}
	}
	var replays map[string]replay
	read(filepath.Join(root, "simulation-replays.json"), &replays)
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
		replayed, ok := replays[m.Algorithm+"-"+m.Environment]
		if !ok {
			panic("missing catalog simulation replay")
		}
		var simulation struct{ Run domain.ExecutionRun }
		replaySHA := read(replayed.Result, &simulation)
		if simulation.Run.Status != domain.ExecutionRunCompleted || simulation.Run.CompletedActivityCount != 58 || simulation.Run.SchedulePlanID != plan.ID || math.Abs(simulation.Run.MakespanSeconds-native.Executed.MakespanSeconds) > 1e-7 {
			panic("catalog simulation does not reproduce the frozen native result")
		}
		if !reflect.DeepEqual(scientificRequest(filepath.Join(capsule, "request.json")), scientificRequest(replayed.Submitted)) {
			panic("catalog simulation input differs from the frozen capsule")
		}
		var raw any
		if read(filepath.Join(capsule, "request.json"), &raw) != m.InputSHA || read(filepath.Join(capsule, "planning-request.json"), &raw) != m.PlanningSHA {
			panic("frozen inputs changed")
		}
		// Prefer a separately executed observation of this exact algorithm's
		// capsule. Fall back to an explicitly audited equivalence group only
		// for the preserved earlier campaign.
		representativeKey := m.Algorithm + "-" + m.Environment
		source, ok := sources[representativeKey]
		if !ok {
			representativeKey = m.Group
			source, ok = sources[representativeKey]
		}
		if !ok {
			panic("missing real deployment " + m.Group)
		}
		var real realTrace
		realPath := filepath.Join(source.Directory, "strict-live-monitor.json")
		historical := source.Historical
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
				EligibleHistoricalReal                     bool `json:"eligibleHistoricalReal"`
			}
			read(filepath.Join(root, "historical-prism-hpc-equivalence.json"), &proof)
			if !proof.Equivalent || !proof.EligibleHistoricalReal || proof.ChangedResource != 0 || proof.ChangedSlot != 0 || proof.ChangedOrder != 0 {
				panic("historical equivalence not proved")
			}
		} else {
			var representative domain.SchedulePlan
			read(filepath.Join(root, "frozen", representativeKey, "plan.json"), &representative)
			if !reflect.DeepEqual(submitted.Plan, representative) || real.Run.SchedulePlanID != representative.ID {
				panic("executed plan differs from frozen representative")
			}
			if !reflect.DeepEqual(scientificRequest(filepath.Join(root, "frozen", representativeKey, "request.json")), scientificRequest(source.Submitted)) {
				panic("real scientific inputs differ from the frozen representative")
			}
			var audit struct {
				AuditVersion int
				Complete     bool
				Violations   []string
				RunID        string
			}
			read(filepath.Join(source.Directory, "strict-order-audit.json"), &audit)
			if audit.AuditVersion < 3 || !audit.Complete || len(audit.Violations) != 0 || audit.RunID != source.RunID {
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
		var simulatedNetwork, realNetwork, executableNetwork int64
		for _, transfer := range native.Transfers {
			simulatedNetwork += transfer.NetworkBytes
		}
		for _, transfer := range real.DataTransfers {
			executable := false
			for _, artifact := range real.ArtifactMaterializations {
				if strings.HasSuffix(transfer.ID, ":transfer-"+artifact.ID) {
					executable = true
					break
				}
			}
			if executable {
				executableNetwork += transfer.NetworkBytes
			} else {
				realNetwork += transfer.NetworkBytes
			}
		}
		if p <= 0 || s <= 0 || r <= 0 {
			panic("invalid makespan")
		}
		lo, hi := math.Min(p, math.Min(s, r)), math.Max(p, math.Max(s, r))
		version := plan.AlgorithmVersion
		relationship := "exact selected plan"
		sharedGroup := ""
		if plan.ID != real.Run.SchedulePlanID {
			relationship = "shared real observation of proved equivalent placement and lane order"
			sharedGroup = m.Group
		}
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
			"simulated_workflow_network_bytes_reported": simulatedNetwork, "real_workflow_network_bytes_reported": realNetwork,
			"real_executable_network_bytes_reported": executableNetwork,
			"transfer_basis":                         "runtime/model counters, not packet capture; workflow network counters exclude executable materialization; aggregate transfer_bytes fields retain original counters",
			"simulated_transfer_seconds":             native.Executed.TransferSeconds, "real_transfer_seconds": real.Run.Breakdown.TransferSeconds,
			"cost_basis":  "scheduler/execution estimates using frozen resource prices; execution cost includes observed allocation idle/disk costs; not a cloud invoice",
			"real_run_id": real.Run.ID, "plan_id": plan.ID, "simulation_run_id": simulation.Run.ID, "frozen_native_run_id": native.RunID, "shared_real_group": sharedGroup, "historical_real_protocol": historical,
			"executed_plan_id": real.Run.SchedulePlanID, "real_plan_relationship": relationship,
			"simulation_replay_source": replayed.Result, "simulation_replay_sha256": replaySHA,
			"real_status": "completed", "boundary": "first activity start (before launch overhead) to final process completion; excludes initial preparation/admission and final workspace publication",
			"cohort":            "frozen calibrated input; observation provenance specified by real run and executed plan identities",
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
	columns = append(columns, "simulated_workflow_network_bytes_reported", "real_workflow_network_bytes_reported", "real_executable_network_bytes_reported", "transfer_basis")
	columns = append(columns, "frozen_native_run_id", "simulation_replay_source", "simulation_replay_sha256")
	columns = append(columns, "executed_plan_id", "real_plan_relationship")
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
