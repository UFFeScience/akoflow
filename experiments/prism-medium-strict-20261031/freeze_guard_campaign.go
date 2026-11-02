//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	planning "github.com/UFFeScience/akoflow/internal/domain/planning"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

type item struct {
	Algorithm, Environment, Directory, InputSHA, PlanSHA, NativeSHA, PlanningSHA, Group string
	Planned, Simulated                                                                  float64
	CandidateOrigin                                                                     any
	Request                                                                             ports.ExecutionRequest `json:"-"`
}

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string, v any) ([]byte, string) {
	b, e := os.ReadFile(p)
	must(e)
	must(json.Unmarshal(b, v))
	return b, fmt.Sprintf("%x", sha256.Sum256(b))
}
func equivalent(a, b domain.SchedulePlan) bool {
	if len(a.Assignments) != len(b.Assignments) {
		return false
	}
	by := map[string]domain.PlanAssignment{}
	for _, x := range a.Assignments {
		by[x.ActivityID] = x
	}
	forward, reverse := map[string]string{}, map[string]string{}
	for _, y := range b.Assignments {
		x, ok := by[y.ActivityID]
		if !ok || x.ResourceID != y.ResourceID || x.SlotID != y.SlotID || x.OrderOnResource != y.OrderOnResource || x.Priority != y.Priority {
			return false
		}
		k, v := x.ResourceID+":"+x.CoreID, y.ResourceID+":"+y.CoreID
		if prior, ok := forward[k]; ok && prior != v {
			return false
		}
		if prior, ok := reverse[v]; ok && prior != k {
			return false
		}
		forward[k], reverse[v] = v, k
	}
	return reflect.DeepEqual(planning.LanePredecessors(a.Assignments), planning.LanePredecessors(b.Assignments))
}
func scientific(r ports.ExecutionRequest) []byte {
	r.Run = domain.ExecutionRun{}
	r.Plan = domain.SchedulePlan{}
	for i := range r.Resources {
		m := map[string]any{}
		for k, v := range r.Resources[i].Metadata {
			if k != "workspaceStorageQuotaOverride" {
				m[k] = v
			}
		}
		r.Resources[i].Metadata = m
	}
	b, e := json.Marshal(r)
	must(e)
	return b
}
func main() {
	root := "outputs/fig8-guard-real-20261101/frozen"
	if _, e := os.Stat(root); !os.IsNotExist(e) {
		panic("freeze output already exists")
	}
	must(os.MkdirAll(root, 0700))
	items := []item{}
	for _, env := range []string{"hpc", "medium"} {
		for _, algo := range []string{"prism-time", "heft", "moheft"} {
			src := "outputs/fig8-prism-heft-guard-final-20261101-" + algo + "-" + env
			dst := filepath.Join(root, algo+"-"+env)
			must(os.Mkdir(dst, 0700))
			var r ports.ExecutionRequest
			raw, ih := read(src+"/simulation-request.json", &r)
			var p domain.SchedulePlan
			pb, ph := read(src+"/selected-plan.json", &p)
			if !reflect.DeepEqual(r.Plan, p) {
				panic("selected and submitted simulation plan differ")
			}
			var trace domain.ExecutionTrace
			tb, th := read(src+"/native-verification/verified-result.json", &trace)
			if trace.PlanID != p.ID || trace.RunID != r.Run.ID || len(trace.Tasks) != 58 {
				panic("native trace identity mismatch")
			}
			assign := map[string]domain.PlanAssignment{}
			for _, a := range p.Assignments {
				assign[a.ActivityID] = a
			}
			seen := map[string]bool{}
			for _, t := range trace.Tasks {
				a, ok := assign[t.ActivityID]
				if !ok || seen[t.ActivityID] || t.PlanAssignmentID != a.ID || t.AllocatedResourceID != a.ResourceID || t.Status != domain.TaskCompleted {
					panic("native trace assignment mismatch")
				}
				seen[t.ActivityID] = true
			}
			var pr any
			prb, prh := read(src+"/planning-request.json", &pr)
			for name, data := range map[string][]byte{"request.json": raw, "plan.json": pb, "native-result.json": tb, "planning-request.json": prb} {
				must(os.WriteFile(filepath.Join(dst, name), data, 0600))
			}
			items = append(items, item{Algorithm: algo, Environment: env, Directory: dst, InputSHA: ih, PlanSHA: ph, NativeSHA: th, PlanningSHA: prh, Planned: p.Predicted.MakespanSeconds, Simulated: trace.Executed.MakespanSeconds, CandidateOrigin: p.Metadata["candidateOrigin"], Request: r})
		}
	}
	for i := range items {
		a := &items[i]
		a.Group = a.Algorithm + "-" + a.Environment
		for j := 0; j < i; j++ {
			b := items[j]
			if a.Environment == b.Environment {
				if a.PlanningSHA != b.PlanningSHA {
					panic("planning input mismatch within environment")
				}
				if equivalent(a.Request.Plan, b.Request.Plan) {
					if !reflect.DeepEqual(scientific(a.Request), scientific(b.Request)) {
						panic("equivalent placement has different scientific execution input")
					}
					if math.Abs(a.Simulated-b.Simulated) > 1e-7 {
						panic("equivalent native timings disagree")
					}
					a.Group = b.Group
					break
				}
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Environment+items[i].Algorithm < items[j].Environment+items[j].Algorithm
	})
	b, e := json.MarshalIndent(items, "", "  ")
	must(e)
	must(os.WriteFile(filepath.Join(root, "manifest.json"), b, 0600))
	fmt.Println(string(b))
}
