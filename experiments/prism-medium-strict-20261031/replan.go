//go:build ignore

// Replan a frozen execution request without modifying the live inventory.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
)

type sink struct{ plans []domain.SchedulePlan }

func (s *sink) Emit(_ context.Context, p domain.SchedulePlan) error {
	s.plans = append(s.plans, p)
	return nil
}

type progress struct{}

func (progress) Report(_ context.Context, fraction float64, message string) error {
	fmt.Printf("%.1f%% %s\n", fraction*100, message)
	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func save(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, b, 0600))
}

func main() {
	if len(os.Args) != 3 {
		panic("usage: replan.go EXECUTION_REQUEST NEW_OUTPUT_DIRECTORY")
	}
	_, err := os.Stat(os.Args[2])
	if !os.IsNotExist(err) {
		panic("output must be new")
	}
	must(os.MkdirAll(os.Args[2], 0700))
	b, err := os.ReadFile(os.Args[1])
	must(err)
	var r ports.ExecutionRequest
	must(json.Unmarshal(b, &r))
	request := domain.PlanningRequest{
		Workflow: r.Workflow, ExecutionScope: r.ExecutionScope,
		Resources: r.Resources, NetworkTopology: r.NetworkTopology,
		ActivityProfiles: r.ActivityProfiles,
		DeadlineSeconds:  r.Plan.DeadlineSeconds, Budget: r.Plan.Budget,
	}
	planner := algorithms.NewPRISMTime()
	configuration := planner.Descriptor().Defaults
	save(filepath.Join(os.Args[2], "planning-request.json"), request)
	save(filepath.Join(os.Args[2], "configuration.json"), configuration)
	s := &sink{}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	must(planner.Schedule(ctx, request, configuration, progress{}, s))
	if len(s.plans) == 0 {
		panic("no plan")
	}
	sort.SliceStable(s.plans, func(i, j int) bool {
		return s.plans[i].Predicted.MakespanSeconds < s.plans[j].Predicted.MakespanSeconds
	})
	save(filepath.Join(os.Args[2], "candidates.json"), s.plans)
	r.Plan = s.plans[0]
	r.Plan.ID = "prism-time-medium-replanned-20261101"
	r.Run.ID = "prism-time-medium-replanned-20261101-simulation"
	r.Run.Mode = "simulation"
	save(filepath.Join(os.Args[2], "selected-plan.json"), r.Plan)
	save(filepath.Join(os.Args[2], "simulation-request.json"), r)
	fmt.Printf("selected planned makespan: %.9f seconds (%d candidates)\n", r.Plan.Predicted.MakespanSeconds, len(s.plans))
}
