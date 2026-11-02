// montage-order-audit observes an execution without submitting or changing it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	planning "github.com/UFFeScience/akoflow/internal/domain/planning"
)

type event struct {
	Type    string    `json:"eventType"`
	At      time.Time `json:"occurredAt"`
	Payload struct {
		ActivityID string `json:"activityId"`
	} `json:"payload"`
}
type monitor struct {
	Run           domain.ExecutionRun     `json:"run"`
	Activities    []domain.TaskExecution  `json:"activities"`
	Events        []event                 `json:"events"`
	Handles       []domain.ActivityHandle `json:"handles"`
	PhysicalNodes map[string]string       `json:"-"`
}
type report struct {
	PhysicalNodesVerified, PhysicalNodesPending                                                  int
	AuditVersion                                                                                 int       `json:"auditVersion"`
	At                                                                                           time.Time `json:"at"`
	RunID                                                                                        string    `json:"runId"`
	Status                                                                                       string    `json:"status"`
	Planned, Started, Completed, Preparing, Running, Failed, VerifiedRelations, PendingRelations int
	Violations                                                                                   []string
	Complete                                                                                     bool
}

func audit(plan domain.SchedulePlan, workflow domain.WorkflowVersion, trace monitor) report {
	r := report{AuditVersion: 3, At: time.Now().UTC(), RunID: trace.Run.ID, Status: string(trace.Run.Status), Planned: len(plan.Assignments), Violations: []string{}}
	if trace.Run.SchedulePlanID != plan.ID {
		r.Violations = append(r.Violations, "executed plan ID differs from frozen plan")
	}
	bindings := map[string]domain.PlanAssignment{}
	for _, a := range plan.Assignments {
		bindings[a.ActivityID] = a
	}
	starts, ends := map[string]time.Time{}, map[string]time.Time{}
	for _, e := range trace.Events {
		id := e.Payload.ActivityID
		if e.Type == "activity.started" {
			if old, ok := starts[id]; !ok || e.At.Before(old) {
				starts[id] = e.At
			}
		}
		if e.Type == "activity.completed" {
			ends[id] = e.At
		}
	}
	seen := map[string]bool{}
	for _, a := range trace.Activities {
		if seen[a.ActivityID] {
			r.Violations = append(r.Violations, "duplicate attempt: "+a.ActivityID)
		}
		seen[a.ActivityID] = true
		b, ok := bindings[a.ActivityID]
		if !ok {
			r.Violations = append(r.Violations, "unplanned activity: "+a.ActivityID)
			continue
		}
		if b.ID == "" || a.PlanAssignmentID != b.ID {
			r.Violations = append(r.Violations, "assignment identity mismatch: "+a.ActivityID)
		}
		if a.PlannedResourceID != b.ResourceID {
			r.Violations = append(r.Violations, "planned resource mismatch: "+a.ActivityID)
		}
		if (a.StartedAt > 0 || a.AllocatedResourceID != "") && a.AllocatedResourceID != b.ResourceID {
			r.Violations = append(r.Violations, "resource mismatch: "+a.ActivityID)
		}
		switch a.Status {
		case domain.TaskCompleted:
			r.Completed++
		case domain.TaskPreparing:
			r.Preparing++
		case domain.TaskRunning:
			r.Running++
		case domain.TaskFailed, domain.TaskCancelled:
			r.Failed++
		}
		if a.StartedAt > 0 {
			r.Started++
		}
	}
	physicalSeen := map[string]bool{}
	for _, handle := range trace.Handles {
		binding, exists := bindings[handle.ActivityID]
		expected := trace.PhysicalNodes[binding.ResourceID]
		if !exists || expected == "" {
			continue
		}
		physicalSeen[handle.ActivityID] = true
		planned, _ := handle.Metadata["plannedNode"].(string)
		allocated, _ := handle.Metadata["allocatedNode"].(string)
		// Native status and node metadata can arrive in successive inspections.
		// Missing evidence is pending, never proof of placement on another node.
		if allocated == "" {
			r.PhysicalNodesPending++
			continue
		}
		if planned != expected || allocated != expected {
			r.Violations = append(r.Violations, "physical node mismatch: "+handle.ActivityID+" expected "+expected+" allocated "+allocated)
		} else {
			r.PhysicalNodesVerified++
		}
	}
	for _, task := range trace.Activities {
		if task.Status == domain.TaskCompleted && trace.PhysicalNodes[bindings[task.ActivityID].ResourceID] != "" && !physicalSeen[task.ActivityID] {
			r.Violations = append(r.Violations, "missing physical node evidence: "+task.ActivityID)
		}
	}
	relations := map[string]map[string]bool{}
	add := func(to, from string) {
		if relations[to] == nil {
			relations[to] = map[string]bool{}
		}
		relations[to][from] = true
	}
	for to, from := range planning.LanePredecessors(plan.Assignments) {
		add(to, from)
	}
	for _, d := range workflow.Dependencies {
		add(d.ActivityID, d.DependsOnActivityID)
	}
	for to, preds := range relations {
		start, ok := starts[to]
		if !ok {
			continue
		}
		for from := range preds {
			end, exists := ends[from]
			if !exists {
				r.PendingRelations++
				continue
			}
			if end.After(start) {
				r.Violations = append(r.Violations, fmt.Sprintf("%s started at %s before %s completed at %s", to, start.Format(time.RFC3339Nano), from, end.Format(time.RFC3339Nano)))
			} else {
				r.VerifiedRelations++
			}
		}
	}
	r.Complete = r.Status == "completed" && r.Completed == r.Planned && r.Started == r.Planned && len(starts) == r.Planned && len(ends) == r.Planned && r.PendingRelations == 0 && r.PhysicalNodesPending == 0 && r.Failed == 0 && len(r.Violations) == 0
	return r
}
func main() {
	api := flag.String("api", "http://127.0.0.1:5173/akoflow-api", "read-only API")
	run := flag.String("run", "", "run ID")
	dir := flag.String("output", "outputs/prism-time-medium-strict-20261031", "existing campaign directory")
	kind := flag.String("case", "medium", "hpc or medium frozen input prefix")
	once := flag.Bool("once", false, "one snapshot")
	snapshot := flag.String("snapshot", "", "audit a preserved monitor JSON; requires --once")
	flag.Parse()
	if *snapshot != "" && !*once {
		panic("preserved snapshot audit requires --once")
	}
	if *kind != "hpc" && *kind != "medium" {
		panic("case must be hpc or medium")
	}
	must := func(e error) {
		if e != nil {
			panic(e)
		}
	}
	var plan domain.SchedulePlan
	var request struct {
		Workflow  domain.WorkflowVersion
		Resources []domain.Resource
	}
	b, e := os.ReadFile(filepath.Join(*dir, *kind+"-plan.json"))
	must(e)
	must(json.Unmarshal(b, &plan))
	b, e = os.ReadFile(filepath.Join(*dir, *kind+"-request.json"))
	must(e)
	must(json.Unmarshal(b, &request))
	physicalNodes := map[string]string{}
	for _, resource := range request.Resources {
		if resource.Type == domain.ResourceHPCMachine {
			node, _ := resource.Metadata["name"].(string)
			if node == "" {
				panic("HPC resource has no frozen physical node identity")
			}
			physicalNodes[resource.ID] = node
		}
	}
	client := http.Client{Timeout: 20 * time.Second}
	completedPending := 0
	for {
		var raw json.RawMessage
		if *snapshot != "" {
			data, err := os.ReadFile(*snapshot)
			must(err)
			raw = data
		} else {
			response, err := client.Get(*api + "/execution-runs/" + *run + "/monitor/")
			must(err)
			if response.StatusCode != 200 {
				response.Body.Close()
				panic(fmt.Sprintf("monitor HTTP %d", response.StatusCode))
			}
			must(json.NewDecoder(response.Body).Decode(&raw))
			response.Body.Close()
		}
		var trace monitor
		must(json.Unmarshal(raw, &trace))
		trace.PhysicalNodes = physicalNodes
		if trace.Run.ID != *run {
			panic("monitor returned a different run ID")
		}
		r := audit(plan, request.Workflow, trace)
		must(os.WriteFile(filepath.Join(*dir, "strict-live-monitor.json"), raw, 0600))
		encoded, e := json.MarshalIndent(r, "", "  ")
		must(e)
		must(os.WriteFile(filepath.Join(*dir, "strict-order-audit.json"), encoded, 0600))
		f, e := os.OpenFile(filepath.Join(*dir, "strict-order-audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		must(e)
		must(json.NewEncoder(f).Encode(r))
		must(f.Close())
		fmt.Printf("%s status=%s completed=%d/%d running=%d preparing=%d verified=%d pending=%d violations=%d\n", r.At.Format(time.RFC3339), r.Status, r.Completed, r.Planned, r.Running, r.Preparing, r.VerifiedRelations, r.PendingRelations, len(r.Violations))
		if len(r.Violations) > 0 {
			for _, v := range r.Violations {
				fmt.Println(v)
			}
			os.Exit(2)
		}
		if r.Complete {
			return
		}
		if r.Status == "failed" || r.Status == "cancelled" {
			os.Exit(3)
		}
		if *once {
			return
		}
		if r.Status == "completed" {
			completedPending++
			if completedPending >= 4 {
				panic("completed run lacks complete audit evidence after observation grace period")
			}
		}
		time.Sleep(15 * time.Second)
	}
}
