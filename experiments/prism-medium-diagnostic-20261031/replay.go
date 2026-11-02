package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
	"os"
	"strconv"
	"time"
)

type sink struct{ Plans []domain.SchedulePlan }

func (s *sink) Emit(_ context.Context, p domain.SchedulePlan) error {
	s.Plans = append(s.Plans, p)
	return nil
}
func main() {
	b, e := os.ReadFile("outputs/fig8-canonical-20261031/prism-time-followup/medium-request.json")
	if e != nil {
		panic(e)
	}
	var req domain.PlanningRequest
	if e = json.Unmarshal(b, &req); e != nil {
		panic(e)
	}
	width, ready := 120, 3
	if len(os.Args) > 1 {
		width, _ = strconv.Atoi(os.Args[1])
	}
	if len(os.Args) > 2 {
		ready, _ = strconv.Atoi(os.Args[2])
	}
	cfg := map[string]any{"beamWidth": width, "readyBranchLimit": ready, "optionCount": 25, "interferenceAware": true}
	out := &sink{}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	e = algorithms.NewPRISMTime().Schedule(ctx, req, cfg, nil, out)
	if e != nil {
		panic(e)
	}
	result := map[string]any{"scope": "offline diagnostic, no physical execution or campaign replacement", "beamWidth": width, "readyBranchLimit": ready, "elapsedSeconds": time.Since(start).Seconds(), "plans": out.Plans}
	data, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		panic(e)
	}
	path := fmt.Sprintf("outputs/prism-medium-investigation-20261031/replay-%d-%d.json", width, ready)
	if e = os.WriteFile(path, data, 0644); e != nil {
		panic(e)
	}
	fmt.Printf("%s plans=%d elapsed=%s\n", path, len(out.Plans), time.Since(start))
	for i, p := range out.Plans {
		if i < 3 {
			fmt.Printf("%d %+v\n", i, p.Predicted)
		}
	}
}
