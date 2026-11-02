// montage-hpc-budget versions simulation inputs after an explicit user decision.
// It never changes a live catalog, provisions resources, or submits executions.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func removeCap(request map[string]any, at string) (int, error) {
	run, ok := request["Run"].(map[string]any)
	if !ok || run["mode"] != "simulation" {
		return 0, fmt.Errorf("simulation-only request required")
	}
	resources, ok := request["Resources"].([]any)
	if !ok {
		return 0, fmt.Errorf("resources missing")
	}
	count := 0
	for _, raw := range resources {
		r, ok := raw.(map[string]any)
		if !ok || r["type"] != "hpc_machine" {
			continue
		}
		m, ok := r["metadata"].(map[string]any)
		if !ok {
			m = map[string]any{}
			r["metadata"] = m
		}
		previous := m["workspaceStorageBudget"]
		delete(m, "workspaceStorageBudget")
		m["workspaceStorageBudgetDecision"] = map[string]any{
			"status": "cap-removed", "source": "explicit-user-decision-20261011",
			"recordedAt": at, "previousBudget": previous,
			"meaning": "No enforced HPC storage cap in this forecast; not a measurement of unlimited physical capacity.",
		}
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("no HPC resources")
	}
	return count, nil
}

func main() {
	in := flag.String("request", "", "existing simulation input")
	out := flag.String("output", "", "new input; refuses overwrite")
	flag.Parse()
	err := func() error {
		data, err := os.ReadFile(*in)
		if err != nil {
			return err
		}
		var request map[string]any
		if err := json.Unmarshal(data, &request); err != nil {
			return err
		}
		n, err := removeCap(request, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		data, err = json.MarshalIndent(request, "", "  ")
		if err != nil {
			return err
		}
		f, err := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Write(data); err != nil {
			return err
		}
		fmt.Printf("Versioned simulation input: %d HPC caps explicitly removed; other resources unchanged.\n", n)
		return nil
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
