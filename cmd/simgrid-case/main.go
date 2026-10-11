// simgrid-case exports a private simulation-only case using the production
// planner/SimGrid contract. It never submits an execution or provisions a VM.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
	"github.com/UFFeScience/akoflow/internal/provider/simgrid"
)

func main() {
	requestPath := flag.String("request", "", "private simulation request JSON")
	outputPath := flag.String("output", "", "new private case directory (must not exist)")
	replan := flag.Bool("prism-time", false, "generate a new offline PRISM-time placement before exporting")
	flag.Parse()
	if err := exportCase(*requestPath, *outputPath, *replan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func export(requestPath, outputPath string) error {
	return exportCase(requestPath, outputPath, false)
}

type planSink struct{ plans []domain.SchedulePlan }

func (s *planSink) Emit(_ context.Context, plan domain.SchedulePlan) error {
	s.plans = append(s.plans, plan)
	return nil
}

func exportCase(requestPath, outputPath string, replan bool) error {
	if requestPath == "" || outputPath == "" {
		return fmt.Errorf("--request and --output required")
	}
	payload, err := os.ReadFile(requestPath)
	if err != nil {
		return err
	}
	var request ports.ExecutionRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return err
	}
	if request.Run.Mode != domain.ExecutionModeSimulation {
		return fmt.Errorf("offline replanning/export requires simulation mode")
	}
	var candidates []domain.SchedulePlan
	if replan {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		sink := &planSink{}
		planning := domain.PlanningRequest{Workflow: request.Workflow, ExecutionScope: request.ExecutionScope,
			Resources: request.Resources, NetworkTopology: request.NetworkTopology, ActivityProfiles: request.ActivityProfiles,
			DeadlineSeconds: request.Plan.DeadlineSeconds, Budget: request.Plan.Budget}
		if err := algorithms.NewPRISMTime().Schedule(ctx, planning, map[string]any{"requireMeasuredProfiles": true}, nil, sink); err != nil {
			return err
		}
		if len(sink.plans) == 0 {
			return fmt.Errorf("no feasible evidenced PRISM-time candidate")
		}
		candidates = sink.plans
		request.Plan = candidates[0]
		request.Run.SchedulePlanID = request.Plan.ID
	}
	platform, input, err := simgrid.BuildOfflineCase(request, 1e9)
	if err != nil {
		return err
	}
	if err := os.Mkdir(outputPath, 0700); err != nil {
		return err
	}
	files := map[string][]byte{"platform.xml": platform, "simulation.json": input}
	if replan {
		files["request.json"], err = json.MarshalIndent(request, "", "  ")
		if err != nil {
			return err
		}
		files["candidates.json"], err = json.MarshalIndent(candidates, "", "  ")
		if err != nil {
			return err
		}
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(outputPath, name), content, 0600); err != nil {
			return err
		}
	}
	return nil
}
