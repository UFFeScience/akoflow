// montage-sla runs an offline, simulation-only SLA sensitivity experiment.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
	"github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/simgrid"
)

type scenario struct{ ID, Name, Source string }

var scenarios = []scenario{{"S1", "Cluster homogeneous", "cluster_homo"}, {"S2", "Cluster heterogeneous", "cluster_hetero"}, {"S3", "Cloud homogeneous", "cloud_homo"}, {"S4", "Cloud heterogeneous", "cloud_hetero"}, {"S5", "Hybrid homogeneous", "hybrid_homo"}, {"S6", "Hybrid heterogeneous", "hybrid_hetero"}, {"S7", "Fog-HPC-cloud", "network_fog_hpc_cloud"}}
var margins = []float64{0.8, 0.9, 1, 1.1, 1.2}

type config struct {
	Root, Source, Binary, Action string
	Beam                         int
}
type campaign struct {
	config
	inputs              map[string]ports.ExecutionRequest
	inputSHA            map[string]string
	executor            *simgrid.ProcessExecutor
	modelSHA, binarySHA string
}
type sink struct{ plans []domain.SchedulePlan }

func (s *sink) Emit(_ context.Context, p domain.SchedulePlan) error {
	s.plans = append(s.plans, p)
	return nil
}

type result struct {
	ID                     string                  `json:"id"`
	Environment            string                  `json:"environment"`
	Panel                  string                  `json:"panel"`
	Algorithm              string                  `json:"algorithm"`
	SLAAware               bool                    `json:"sla_aware"`
	Margin                 float64                 `json:"margin"`
	Deadline               float64                 `json:"deadline_seconds"`
	Budget                 float64                 `json:"budget_usd"`
	BudgetReference        string                  `json:"budget_reference"`
	BudgetInformative      bool                    `json:"budget_informative"`
	Predicted              domain.PredictedMetrics `json:"predicted"`
	Simulated              domain.ExecutionMetrics `json:"simulated"`
	DeadlineMet            bool                    `json:"deadline_met"`
	BudgetMet              bool                    `json:"budget_met"`
	SLAMet                 bool                    `json:"sla_met"`
	PredictionSaysFeasible bool                    `json:"predicted_sla_met"`
	BestEffort             bool                    `json:"best_effort"`
	CandidateCount         int                     `json:"candidate_count"`
	EligibleCandidates     int                     `json:"eligible_candidates"`
	PlanID                 string                  `json:"plan_id"`
	RunID                  string                  `json:"run_id"`
	PlanPath               string                  `json:"plan_path"`
	PlanSHA                string                  `json:"plan_sha256"`
	TracePath              string                  `json:"trace_path"`
	TraceSHA               string                  `json:"trace_sha256"`
	NativePath             string                  `json:"native_path"`
	NativeSHA              string                  `json:"native_sha256"`
	InputSHA               string                  `json:"input_sha256"`
	ModelSHA               string                  `json:"model_sha256"`
	BinarySHA              string                  `json:"simgrid_binary_sha256"`
	ScheduleSHA            string                  `json:"schedule_sha256"`
	PlanningSeconds        float64                 `json:"planning_wall_seconds"`
	SimulationSeconds      float64                 `json:"simulation_wall_seconds"`
	Completed              int                     `json:"completed_activities"`
	VerifiedRelations      int                     `json:"verified_relations"`
	CandidateOrigin        any                     `json:"candidate_origin,omitempty"`
}

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func digest(b []byte) string  { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func fileSHA(p string) string { b, e := os.ReadFile(p); must(e); return digest(b) }
func save(p string, x any) {
	b, e := json.Marshal(x)
	must(e)
	must(os.MkdirAll(filepath.Dir(p), 0700))
	f, e := os.CreateTemp(filepath.Dir(p), ".partial-*")
	must(e)
	defer os.Remove(f.Name())
	_, e = f.Write(b)
	must(e)
	must(f.Sync())
	must(f.Close())
	must(os.Rename(f.Name(), p))
}
func load(p string, x any) bool {
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return false
	}
	must(e)
	must(json.Unmarshal(b, x))
	return true
}
func logf(format string, args ...any) {
	fmt.Printf(time.Now().UTC().Format(time.RFC3339)+" "+format+"\n", args...)
}
func modelHash() string {
	paths := []string{}
	for _, root := range []string{"internal/domain", "internal/planning/algorithms", "internal/planning/moheftplugin", "internal/provider/simgrid"} {
		must(filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				paths = append(paths, p)
			}
			return nil
		}))
	}
	sort.Strings(paths)
	var all strings.Builder
	for _, p := range paths {
		all.WriteString(p + ":" + fileSHA(p) + "\n")
	}
	return digest([]byte(all.String()))
}
func prepare(c config) *campaign {
	must(os.MkdirAll(c.Root, 0700))
	executor, e := simgrid.NewProcessExecutor(provider.OSCommandExecutor{}, simgrid.ProcessConfig{BinaryPath: c.Binary, Workspace: filepath.Join(c.Root, "native"), MaxConcurrent: 1, Timeout: 30 * time.Minute, ReferenceFLOPS: 1e9})
	must(e)
	x := &campaign{config: c, inputs: map[string]ports.ExecutionRequest{}, inputSHA: map[string]string{}, executor: executor, modelSHA: modelHash(), binarySHA: fileSHA(c.Binary)}
	workflowSHA := ""
	frozen := []map[string]any{}
	for _, s := range scenarios {
		source := filepath.Join(c.Source, s.Source, "time-request.json")
		b, e := os.ReadFile(source)
		must(e)
		var r ports.ExecutionRequest
		must(json.Unmarshal(b, &r))
		if r.Run.Mode != domain.ExecutionModeSimulation || len(r.Workflow.Activities) != 6448 || len(r.Workflow.Dependencies) != 18924 {
			panic("requires the full simulation-only Montage6448 paper snapshot")
		}
		r.Plan = domain.SchedulePlan{}
		r.Run = domain.ExecutionRun{ID: "frozen-input-" + s.ID, Mode: domain.ExecutionModeSimulation, Seed: 911}
		w, e := json.Marshal(r.Workflow)
		must(e)
		if workflowSHA != "" && workflowSHA != digest(w) {
			panic("workflow differs across environments")
		}
		workflowSHA = digest(w)
		p := filepath.Join(c.Root, "frozen", s.ID+".json")
		if _, e := os.Stat(p); os.IsNotExist(e) {
			save(p, r)
		} else {
			var previous ports.ExecutionRequest
			load(p, &previous)
			a, _ := json.Marshal(previous)
			z, _ := json.Marshal(r)
			if digest(a) != digest(z) {
				panic("frozen inputs changed")
			}
		}
		x.inputs[s.ID] = r
		x.inputSHA[s.ID] = fileSHA(p)
		frozen = append(frozen, map[string]any{"environment": s.ID, "name": s.Name, "source": source, "source_sha256": digest(b), "input": p, "input_sha256": x.inputSHA[s.ID], "resources": len(r.Resources), "activities": 6448, "dependencies": 18924})
	}
	identity := map[string]any{"schema": 1, "workflow_sha256": workflowSHA, "model_sha256": x.modelSHA, "simgrid_binary_sha256": x.binarySHA, "beam_width": c.Beam, "option_count": 25, "ready_branch_limit": 3, "moheft_frontier_limit": 25, "seed": 911, "margins": margins, "scope": "simulation-only", "budget_fallback": "fresh HEFT S4 simulated cost when own-environment HEFT cost is zero", "main_cases": 210, "additional_sla_off_cases": 70, "inputs": frozen}
	var previous map[string]any
	if load(filepath.Join(c.Root, "manifest.json"), &previous) {
		a, _ := json.Marshal(previous)
		b, _ := json.Marshal(identity)
		if digest(a) != digest(b) {
			panic("campaign manifest/model changed; use a new output root")
		}
	} else {
		save(filepath.Join(c.Root, "manifest.json"), identity)
	}
	return x
}
func planning(r ports.ExecutionRequest, d, b float64) domain.PlanningRequest {
	return domain.PlanningRequest{Workflow: r.Workflow, ExecutionScope: r.ExecutionScope, Resources: r.Resources, NetworkTopology: r.NetworkTopology, ActivityProfiles: r.ActivityProfiles, DeadlineSeconds: d, Budget: b}
}
func (c *campaign) baseline(ctx context.Context, s scenario) result {
	path := filepath.Join(c.Root, "baselines", s.ID, "result.json")
	var previous result
	if load(path, &previous) {
		c.checkResult(previous)
		logf("%s HEFT baseline already verified T=%.6f C=%.9f", s.ID, previous.Simulated.MakespanSeconds, previous.Simulated.Cost)
		return previous
	}
	logf("%s HEFT baseline planning", s.ID)
	started := time.Now()
	out := &sink{}
	must((algorithms.HEFT{}).Schedule(ctx, planning(c.inputs[s.ID], 0, 0), nil, nil, out))
	if len(out.plans) != 1 {
		panic("HEFT must return one plan")
	}
	p := out.plans[0]
	r := c.execute(ctx, s, "baseline", "heft", false, 1, 0, 0, p, 1, 1, false, time.Since(started).Seconds())
	save(path, r)
	logf("%s HEFT baseline complete T=%.6f C=%.9f native=%.2fs", s.ID, r.Simulated.MakespanSeconds, r.Simulated.Cost, r.SimulationSeconds)
	return r
}
func main() {
	var c config
	flag.StringVar(&c.Root, "output", "outputs/montage6448-sla-20261101", "private campaign directory")
	flag.StringVar(&c.Source, "source", "outputs/moheft-paper-campaign/montage-6448", "frozen paper execution requests")
	flag.StringVar(&c.Binary, "simgrid-binary", "/usr/local/bin/akoflow-simgrid-runner", "native local SimGrid runner")
	flag.StringVar(&c.Action, "action", "all", "baselines, campaign, verify, or all")
	flag.IntVar(&c.Beam, "beam-width", 20, "fixed PRISM beam width for all paired conditions")
	flag.Parse()
	x := prepare(c)
	ctx := context.Background()
	switch c.Action {
	case "baselines":
		for _, s := range scenarios {
			x.baseline(ctx, s)
		}
	case "campaign", "all":
		x.run(ctx)
	case "verify":
		x.verify()
	default:
		panic("unknown action")
	}
}
