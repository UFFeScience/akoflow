package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
	"github.com/UFFeScience/akoflow/internal/planning/library"
	"github.com/UFFeScience/akoflow/internal/planning/moheftplugin"
	_ "github.com/mattn/go-sqlite3"
)

type cachedPlans struct {
	InputSHA, ModelSHA, PlansSHA string
	PlanningSeconds              float64
	Count                        int
}
type candidateScore struct {
	ID, SHA   string
	Predicted domain.PredictedMetrics
	Eligible  bool
}

func selectPlan(plans []domain.SchedulePlan, objective string, d, b float64, filter bool) (domain.SchedulePlan, int, bool) {
	if len(plans) == 0 {
		panic("no scheduling candidates")
	}
	eligible := []domain.SchedulePlan{}
	for _, p := range plans {
		if (d <= 0 || within(p.Predicted.MakespanSeconds, d)) && (b <= 0 || within(p.Predicted.Cost, b)) {
			eligible = append(eligible, p)
		}
	}
	pool := append([]domain.SchedulePlan(nil), plans...)
	if filter && len(eligible) > 0 {
		pool = eligible
	}
	sort.SliceStable(pool, func(i, j int) bool {
		a, z := pool[i].Predicted, pool[j].Predicted
		if objective == "cost" {
			if a.Cost != z.Cost {
				return a.Cost < z.Cost
			}
			if a.MakespanSeconds != z.MakespanSeconds {
				return a.MakespanSeconds < z.MakespanSeconds
			}
		} else {
			if a.MakespanSeconds != z.MakespanSeconds {
				return a.MakespanSeconds < z.MakespanSeconds
			}
			if a.Cost != z.Cost {
				return a.Cost < z.Cost
			}
		}
		return pool[i].ID < pool[j].ID
	})
	return pool[0], len(eligible), len(eligible) == 0
}
func (c *campaign) configuration(aware bool) map[string]any {
	return map[string]any{"beamWidth": c.Beam, "optionCount": 25, "readyBranchLimit": 3, "interferenceAware": true, "slaAware": aware}
}

func (c *campaign) schedulerConfiguration(algorithm string, aware bool) map[string]any {
	if algorithm == "heft" {
		return map[string]any{}
	}
	if strings.HasPrefix(algorithm, "moheft-") {
		return map[string]any{"candidateCount": 25}
	}
	return c.configuration(aware)
}
func (c *campaign) cached(ctx context.Context, s scenario, key string, scheduler ports.Scheduler, configuration map[string]any) ([]domain.SchedulePlan, float64) {
	dir := filepath.Join(c.Root, "planner-cache", s.ID, key)
	path := filepath.Join(dir, "plans.json")
	var meta cachedPlans
	var plans []domain.SchedulePlan
	if load(filepath.Join(dir, "metadata.json"), &meta) {
		if meta.InputSHA != c.inputSHA[s.ID] || meta.ModelSHA != c.modelSHA || meta.PlansSHA != fileSHA(path) || !load(path, &plans) || len(plans) != meta.Count {
			panic("cached planner identity mismatch")
		}
		return plans, meta.PlanningSeconds
	}
	logf("%s %s generate unconstrained candidates", s.ID, key)
	started := time.Now()
	out := &sink{}
	planCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	must(scheduler.Schedule(planCtx, planning(c.inputs[s.ID], 0, 0), configuration, nil, out))
	if len(out.plans) == 0 {
		panic("empty planner output")
	}
	save(path, out.plans)
	meta = cachedPlans{InputSHA: c.inputSHA[s.ID], ModelSHA: c.modelSHA, PlansSHA: fileSHA(path), PlanningSeconds: time.Since(started).Seconds(), Count: len(out.plans)}
	save(filepath.Join(dir, "metadata.json"), meta)
	return out.plans, meta.PlanningSeconds
}
func (c *campaign) plugin(ctx context.Context) ports.Scheduler {
	db, e := sql.Open("sqlite3", filepath.Join(c.Root, "plugin-catalog.db"))
	must(e)
	registry, e := algorithms.NewRegistry(algorithms.HEFT{})
	must(e)
	service, e := library.New(ctx, db, registry, filepath.Join(c.Root, "plugin-cache"), false)
	must(e)
	if scheduler, ok := registry.Find("moheft"); ok {
		return scheduler
	}
	pkg, e := moheftplugin.Package()
	must(e)
	saved, e := service.Save(ctx, pkg)
	must(e)
	if saved.Status != "ready" {
		panic("MOHEFT compilation failed: " + saved.CompileError)
	}
	save(filepath.Join(c.Root, "moheft-package.json"), saved)
	scheduler, ok := registry.Find("moheft")
	if !ok {
		panic("compiled MOHEFT not registered")
	}
	return scheduler
}
func (c *campaign) run(ctx context.Context) {
	baselines := map[string]result{}
	for _, s := range scenarios {
		baselines[s.ID] = c.baseline(ctx, s)
	}
	fallback := baselines["S4"].Simulated.Cost
	if fallback <= 0 {
		panic("fresh S4 HEFT must provide a positive budget fallback")
	}
	save(filepath.Join(c.Root, "baseline-references.json"), baselines)
	plugin := c.plugin(ctx)
	completed := 0
	for _, s := range scenarios {
		mo, moSeconds := c.cached(ctx, s, "moheft-frontier", plugin, map[string]any{"candidateCount": 25})
		off := map[string][]domain.SchedulePlan{}
		offSeconds := map[string]float64{}
		for _, objective := range []string{"time", "cost"} {
			scheduler := algorithms.NewPRISMTime()
			if objective == "cost" {
				scheduler = algorithms.NewPRISMCost()
			}
			off[objective], offSeconds[objective] = c.cached(ctx, s, "prism-"+objective+"-sla-off", scheduler, c.configuration(false))
		}
		for _, panel := range []string{"time", "cost"} {
			for _, margin := range margins {
				base := baselines[s.ID]
				budgetBase := base.Simulated.Cost
				budgetReference := s.ID
				if budgetBase <= 0 {
					budgetBase = fallback
					budgetReference = "S4-positive-fallback"
				}
				d, b := base.Simulated.MakespanSeconds, budgetBase
				if panel == "time" {
					d *= margin
				} else {
					b *= margin
				}
				for _, algorithm := range []string{"heft", "moheft-" + panel, "prism-" + panel, "prism-" + panel + "-off"} {
					aware := strings.HasPrefix(algorithm, "prism-") && !strings.HasSuffix(algorithm, "-off")
					actual := strings.TrimSuffix(algorithm, "-off")
					variant := ""
					if strings.HasPrefix(actual, "prism-") {
						if aware {
							variant = "-on"
						} else {
							variant = "-off"
						}
					}
					key := fmt.Sprintf("%s-%s-%03d-%s%s", s.ID, panel, int(margin*100+0.5), actual, variant)
					path := filepath.Join(c.Root, "cases", key, "result.json")
					var prior result
					if load(path, &prior) {
						c.checkResult(prior)
						completed++
						continue
					}
					save(filepath.Join(c.Root, "progress.json"), map[string]any{"completed": completed, "expected": 280, "case": key, "phase": "planning", "at": time.Now().UTC()})
					logf("[%d/280] %s D=%.6f B=%.9f planning", completed+1, key, d, b)
					var plans []domain.SchedulePlan
					planningSeconds := 0.0
					switch {
					case actual == "heft":
						var p domain.SchedulePlan
						load(base.PlanPath, &p)
						plans = []domain.SchedulePlan{p}
					case strings.HasPrefix(actual, "moheft-"):
						plans = mo
						planningSeconds = moSeconds
					case !aware:
						plans = off[panel]
						planningSeconds = offSeconds[panel]
					default:
						started := time.Now()
						scheduler := algorithms.NewPRISMTime()
						if panel == "cost" {
							scheduler = algorithms.NewPRISMCost()
						}
						out := &sink{}
						planCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
						e := scheduler.Schedule(planCtx, planning(c.inputs[s.ID], d, b), c.configuration(true), nil, out)
						cancel()
						must(e)
						plans = out.plans
						planningSeconds = time.Since(started).Seconds()
					}
					selected, eligible, bestEffort := selectPlan(plans, panel, d, b, actual != "heft" && (aware || strings.HasPrefix(actual, "moheft-")))
					scores := []candidateScore{}
					for _, p := range plans {
						encoded, e := json.Marshal(p)
						must(e)
						scores = append(scores, candidateScore{ID: p.ID, SHA: digest(encoded), Predicted: p.Predicted, Eligible: within(p.Predicted.MakespanSeconds, d) && within(p.Predicted.Cost, b)})
					}
					save(filepath.Join(c.Root, "cases", key, "candidate-selection.json"), map[string]any{"configuration": c.schedulerConfiguration(actual, aware), "objective": panel, "selected_original_id": selected.ID, "eligible_count": eligible, "candidate_count": len(plans), "filter_predicted_sla": actual != "heft" && (aware || strings.HasPrefix(actual, "moheft-")), "selected_before_native_replay": true, "candidates": scores})
					save(filepath.Join(c.Root, "progress.json"), map[string]any{"completed": completed, "expected": 280, "case": key, "phase": "native-simulation", "at": time.Now().UTC()})
					r := c.execute(ctx, s, panel, actual, aware, margin, d, b, selected, len(plans), eligible, bestEffort, planningSeconds)
					r.BudgetReference = budgetReference
					r.BudgetInformative = monetaryScope(c.inputs[s.ID].Resources)
					save(path, r)
					completed++
					save(filepath.Join(c.Root, "progress.json"), map[string]any{"completed": completed, "expected": 280, "case": key, "phase": "completed-case", "at": time.Now().UTC()})
					logf("[%d/280] %s complete T=%.6f C=%.9f SLA=%v predicted=%v", completed, key, r.Simulated.MakespanSeconds, r.Simulated.Cost, r.SLAMet, r.PredictionSaysFeasible)
				}
			}
		}
	}
	c.verify()
}
func monetaryScope(resources []domain.Resource) bool {
	for _, r := range resources {
		if r.PricePerSecond > 0 {
			return true
		}
	}
	return false
}
