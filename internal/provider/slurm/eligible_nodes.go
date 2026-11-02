package slurm

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Discover within the requested partition, never across heterogeneous families.
func (a *Adapter) eligibleNodes(ctx context.Context, partition, original string, resource domain.Resource) (string, string, string, error) {
	if partition == "" {
		return "", "", "", fmt.Errorf("eligible-node fallback requires an explicit partition")
	}
	originalOutput, err := a.executor.Run(ctx, "scontrol", []string{"show", "node", original, "--oneliner"}, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("inspect planned node: %w", err)
	}
	fields := slurmFields(string(originalOutput))
	features := fields["AvailableFeatures"]
	if fields["NodeName"] != original || features == "" || features == "(null)" {
		return "", "", "", fmt.Errorf("planned node %q has no verified feature set", original)
	}
	for _, feature := range strings.Split(features, ",") {
		if feature == "" || shellToken(feature) != feature {
			return "", "", "", fmt.Errorf("planned node %q has unsupported feature syntax", original)
		}
	}
	output, err := a.executor.Run(ctx, "sinfo", []string{"--partition", partition, "--Node", "--noheader", "--format=%N|%c|%m|%f|%t"}, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("discover eligible nodes: %w", err)
	}
	nodes, err := selectEligibleNodes(string(output), features, resource)
	if err != nil {
		return "", "", "", err
	}
	allowed := map[string]bool{}
	for _, node := range strings.Split(nodes, ",") {
		allowed[node] = true
	}
	excluded := []string{}
	seen := map[string]bool{}
	for _, row := range strings.Split(string(output), "\n") {
		cells := strings.Split(strings.TrimSpace(row), "|")
		// The SSH executor merges stderr into stdout. Ignore diagnostic lines,
		// just as selectEligibleNodes does; only complete inventory rows name
		// nodes. A malformed name within an inventory row still fails closed.
		if len(cells) != 5 {
			continue
		}
		name := strings.TrimSpace(cells[0])
		if name == "" {
			continue
		}
		if shellToken(name) != name {
			return "", "", "", fmt.Errorf("invalid node inventory name")
		}
		if !allowed[name] && !seen[name] {
			excluded = append(excluded, name)
			seen[name] = true
		}
	}
	sort.Strings(excluded)
	return nodes, features, strings.Join(excluded, ","), nil
}

func slurmFields(output string) map[string]string {
	result := map[string]string{}
	for _, field := range strings.Fields(output) {
		key, value, ok := strings.Cut(field, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

func selectEligibleNodes(output, features string, resource domain.Resource) (string, error) {
	nodes := map[string]bool{}
	for _, row := range strings.Split(output, "\n") {
		cells := strings.Split(strings.TrimSpace(row), "|")
		if len(cells) != 5 {
			continue
		}
		cpus, errCPU := strconv.Atoi(cells[1])
		memory, errMemory := strconv.ParseInt(cells[2], 10, 64)
		if errCPU != nil || errMemory != nil || cpus < resource.CPUCores || memory < (resource.MemoryBytes+(1<<20)-1)/(1<<20) {
			continue
		}
		state := strings.TrimRight(cells[4], "~#$@+")
		if state != "idle" && state != "mix" && state != "alloc" {
			continue
		}
		compatible := true
		for _, feature := range strings.Split(features, ",") {
			if !strings.Contains(","+cells[3]+",", ","+feature+",") {
				compatible = false
			}
		}
		if compatible && shellToken(cells[0]) == cells[0] && cells[0] != "" {
			nodes[cells[0]] = true
		}
	}
	selected := []string{}
	for node := range nodes {
		selected = append(selected, node)
	}
	sort.Strings(selected)
	if len(selected) == 0 {
		return "", fmt.Errorf("no eligible nodes preserve the planned resource characteristics")
	}
	return strings.Join(selected, ","), nil
}
