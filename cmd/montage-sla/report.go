package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

func (c *campaign) verify() {
	paths, e := filepath.Glob(filepath.Join(c.Root, "cases", "*", "result.json"))
	must(e)
	rows := []result{}
	main, off := 0, 0
	for _, p := range paths {
		var r result
		if !load(p, &r) {
			panic("missing result")
		}
		c.checkResult(r)
		if r.Panel != "time" && r.Panel != "cost" {
			panic("unexpected panel")
		}
		if r.Deadline <= 0 || r.Budget <= 0 || math.IsNaN(r.Simulated.MakespanSeconds) || math.IsNaN(r.Simulated.Cost) || r.Simulated.MakespanSeconds <= 0 || r.Simulated.Cost < 0 {
			panic("invalid completed scenario")
		}
		if r.SLAMet != (within(r.Simulated.MakespanSeconds, r.Deadline) && within(r.Simulated.Cost, r.Budget)) {
			panic("SLA classification mismatch")
		}
		if strings.HasPrefix(r.Algorithm, "prism-") && !r.SLAAware {
			off++
		} else {
			main++
		}
		rows = append(rows, r)
	}
	if main != 210 || off != 70 {
		panic(fmt.Sprintf("incomplete campaign main=%d/210 off=%d/70", main, off))
	}
	save(filepath.Join(c.Root, "results.json"), rows)
	save(filepath.Join(c.Root, "coverage-audit.json"), map[string]any{"complete": true, "main_cases": main, "additional_sla_off_cases": off, "native_simulations": len(rows), "activities_each": 6448, "simulation_only": true})
	logf("verified main=%d off=%d native traces complete", main, off)
}
