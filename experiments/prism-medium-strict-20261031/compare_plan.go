//go:build ignore

// Compare executable placement and lane order, excluding predictions and IDs.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainplanning "github.com/UFFeScience/akoflow/internal/domain/planning"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// Explicit compatibility aliases: Figure 8 inventory IDs wrap the same
// persisted physical resources. No fuzzy matching of machines or cores.
func physical(id string) string {
	for _, prefix := range []string{"fig8-hpc-", "fig8-medium-"} {
		id = strings.TrimPrefix(id, prefix)
	}
	return id
}

func main() {
	if len(os.Args) != 4 {
		panic("OLD_EXECUTION_REQUEST NEW_PLAN REAL_TRACE")
	}
	load := func(path string, value any) string {
		b, err := os.ReadFile(path)
		check(err)
		check(json.Unmarshal(b, value))
		return fmt.Sprintf("%x", sha256.Sum256(b))
	}
	var old ports.ExecutionRequest
	var next domain.SchedulePlan
	oldHash := load(os.Args[1], &old)
	newHash := load(os.Args[2], &next)
	var real struct {
		Run struct {
			ID, Status      string
			MakespanSeconds float64
		}
		Activities []struct{ ActivityID, PlanAssignmentID, PlannedResourceID string }
	}
	realHash := load(os.Args[3], &real)
	if len(old.Plan.Assignments) != 58 || len(next.Assignments) != 58 || len(real.Activities) != 58 || real.Run.Status != "completed" {
		panic("expected completed 58 activity reference")
	}
	before := map[string]domain.PlanAssignment{}
	for _, a := range old.Plan.Assignments {
		before[a.ActivityID] = a
	}
	for _, a := range real.Activities {
		p, ok := before[a.ActivityID]
		if !ok || p.ID != a.PlanAssignmentID || physical(p.ResourceID) != physical(a.PlannedResourceID) {
			panic("old request does not match real trace")
		}
	}
	differences := []map[string]any{}
	resources, cores, orders, priorities := 0, 0, 0, 0
	slots := 0
	coreForward, coreReverse := map[string]string{}, map[string]string{}
	corePermutation := true
	for _, n := range next.Assignments {
		o, ok := before[n.ActivityID]
		if !ok {
			panic("workflow activity mismatch")
		}
		r, c, q, p := physical(o.ResourceID) != physical(n.ResourceID), physical(o.CoreID) != physical(n.CoreID), o.OrderOnResource != n.OrderOnResource, o.Priority != n.Priority
		slotChanged := o.SlotID != n.SlotID
		if slotChanged {
			slots++
		}
		oldCore, newCore := physical(o.ResourceID)+":"+physical(o.CoreID), physical(n.ResourceID)+":"+physical(n.CoreID)
		if prior, ok := coreForward[oldCore]; ok && prior != newCore {
			corePermutation = false
		}
		if prior, ok := coreReverse[newCore]; ok && prior != oldCore {
			corePermutation = false
		}
		coreForward[oldCore], coreReverse[newCore] = newCore, oldCore
		if r {
			resources++
		}
		if c {
			cores++
		}
		if q {
			orders++
		}
		if p {
			priorities++
		}
		if r || c || q || p || slotChanged {
			differences = append(differences, map[string]any{"activity": n.ActivityID, "oldResource": physical(o.ResourceID), "newResource": physical(n.ResourceID), "oldCore": physical(o.CoreID), "newCore": physical(n.CoreID), "oldOrder": o.OrderOnResource, "newOrder": n.OrderOnResource, "priorityChanged": p})
		}
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i]["activity"].(string) < differences[j]["activity"].(string) })
	sameLanes := reflect.DeepEqual(domainplanning.LanePredecessors(old.Plan.Assignments), domainplanning.LanePredecessors(next.Assignments))
	result := map[string]any{"realRunId": real.Run.ID, "realSeconds": real.Run.MakespanSeconds, "oldRequestSHA256": oldHash, "newPlanSHA256": newHash, "realTraceSHA256": realHash, "changedResource": resources, "changedCore": cores, "changedOrder": orders, "changedPriority": priorities, "changedActivities": len(differences), "sameExecutablePlan": len(differences) == 0, "coreRelabelingIsBijection": corePermutation, "sameLanePrecedences": sameLanes, "equivalentUpToIdenticalCoreLabels": resources == 0 && priorities == 0 && sameLanes && corePermutation, "differences": differences}
	result["changedSlot"] = slots
	result["equivalentUpToIdenticalCoreLabels"] = resources == 0 && priorities == 0 && slots == 0 && sameLanes && corePermutation
	b, err := json.MarshalIndent(result, "", "  ")
	check(err)
	fmt.Println(string(b))
}
