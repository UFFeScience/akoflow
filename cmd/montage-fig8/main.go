// montage-fig8 prepares isolated Figure 8 infrastructure snapshots.
// It does not delete infrastructure or submit paid executions.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
)

type obj = map[string]any

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func read(path string, v any) { b, e := os.ReadFile(path); must(e); must(json.Unmarshal(b, v)) }
func save(path string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	must(e)
	must(os.WriteFile(path, b, 0600))
}
func remap(v any, ids map[string]string) any {
	switch x := v.(type) {
	case string:
		if y, ok := ids[x]; ok {
			return y
		}
		return x
	case []any:
		for i, y := range x {
			x[i] = remap(y, ids)
		}
		return x
	case map[string]any:
		r := obj{}
		for k, y := range x {
			nk := k
			if s, ok := ids[k]; ok {
				nk = s
			}
			r[nk] = remap(y, ids)
		}
		return r
	default:
		return v
	}
}
func clone[T any](v T) T {
	b, e := json.Marshal(v)
	must(e)
	var out T
	must(json.Unmarshal(b, &out))
	return out
}
func build(source ports.ExecutionRequest, hpc, cloud domain.EnvironmentDefinition, targets []obj, kind, ip string) (domain.EnvironmentDefinition, ports.ExecutionRequest, []obj) {
	prefix := "fig8-" + kind
	env := clone(hpc)
	env.Environment.ID = prefix
	env.Environment.Name = "Figure 8 · " + kind
	env.Environment.Description = "Montage 58 HEFT; exact Figure 8 capacity; calibrated October 2026 inputs."
	env.Version.ID = prefix + "-v1"
	env.Version.EnvironmentID = prefix
	env.Version.ConfigurationHash = prefix + "-v1"
	env.Resources = nil
	env.RuntimeBindings = nil
	env.Profiles = nil
	env.ConnectionChecks = nil
	if kind != "hpc" {
		env.Connections = append(env.Connections, clone(cloud.Connections)...)
		env.Runtimes = append(env.Runtimes, clone(cloud.Runtimes)...)
	}
	allowed := map[string]bool{"plafrim-execution-ssh-node-diablo03": true, "montage-relay-controller": true}
	for _, r := range source.Resources {
		if kind == "micro" && strings.Contains(r.ID, "e2-micro-") || kind == "medium" && strings.Contains(r.ID, "e2-medium-") {
			allowed[r.ID] = true
		}
	}
	ids := map[string]string{"plafrim-execution-v1": env.Version.ID, "gcp-environment-v1": env.Version.ID, "plafrim-execution": prefix, "gcp-environment": prefix}
	for _, r := range source.Resources {
		if allowed[r.ID] {
			ids[r.ID] = prefix + "-" + r.ID
		}
	}
	for _, r := range env.Runtimes {
		ids[r.ID] = prefix + "-" + r.ID
	}
	for _, c := range env.Connections {
		ids[c.ID] = prefix + "-" + c.ID
	}
	for _, storage := range env.Storages {
		ids[storage.ID] = prefix + "-" + storage.ID
	}
	for _, r := range source.Resources {
		if allowed[r.ID] {
			r = clone(r)
			r.EnvironmentVersionID = env.Version.ID
			if r.Type == domain.ResourceHPCMachine && kind != "hpc" {
				r.CPUCapacity = 1
			}
			r.Metadata["workspaceStorageLifecycle"] = "consumer-barrier-v1"
			delete(r.Metadata, "workspaceStorageBudgetDecision")
			env.Resources = append(env.Resources, r)
		}
	}
	for _, p := range source.ActivityProfiles {
		if allowed[p.ResourceID] {
			p.ID = prefix + "-" + p.ID
			env.Profiles = append(env.Profiles, p)
		}
	}
	for _, r := range env.Resources {
		if !r.Schedulable {
			continue
		}
		runtime := "gcp-environment-cloud"
		if r.Type == domain.ResourceHPCMachine {
			runtime = "plafrim-execution-slurm"
		}
		env.RuntimeBindings = append(env.RuntimeBindings, domain.ResourceRuntimeBinding{ResourceID: r.ID, RuntimeID: runtime, Enabled: true})
	}
	sim := domain.EnvironmentRuntime{ID: prefix + "-simgrid", EnvironmentVersionID: env.Version.ID, Name: "SimGrid", Driver: "simgrid", Mode: "simulation", Capabilities: domain.EnvironmentCapabilities{Simulation: true}}
	env.Runtimes = append(env.Runtimes, sim)
	for _, r := range env.Resources {
		if r.Schedulable {
			env.RuntimeBindings = append(env.RuntimeBindings, domain.ResourceRuntimeBinding{ResourceID: r.ID, RuntimeID: sim.ID, Enabled: true})
		}
	}
	for i := range env.Connections {
		c := &env.Connections[i]
		if c.Type == "ssh" {
			c.Configuration["eligibleNodeFallback"] = false
		}
	}
	// Filter the topology before changing identifiers, including model-only relay.
	top := clone(source.NetworkTopology)
	top.ID = prefix + "-network"
	top.ExecutionScopeID = prefix + "-scope"
	top.Links = nil
	for _, l := range source.NetworkTopology.Links {
		if allowed[l.SourceResourceID] && allowed[l.TargetResourceID] {
			l.ID = prefix + "-" + l.ID
			l.TopologyID = top.ID
			top.Links = append(top.Links, l)
		}
	}
	if model, ok := top.Metadata["workspaceMaterializationModel"].(map[string]any); ok {
		// The archived October input already contains v2 fixed-stage fields,
		// but incorrectly declares v1. Preserve all coefficients and provenance.
		model["schemaVersion"] = 2
		top.Metadata["schemaMigration"] = obj{"from": 1, "to": 2, "reason": "declare existing fixed stage service fields; coefficients unchanged", "source": "montage-fig8"}
		if rs, ok := model["resources"].(map[string]any); ok {
			for id := range rs {
				if !allowed[id] {
					delete(rs, id)
				}
			}
		}
		encoded, e := json.Marshal(model)
		must(e)
		var validated materializationmodel.Model
		must(json.Unmarshal(encoded, &validated))
		must(validated.Validate())
	}
	scope := domain.ExecutionScope{ID: prefix + "-scope", Name: env.Environment.Name, NetworkTopologyID: top.ID, EnvironmentVersionIDs: []string{env.Version.ID}, Metadata: obj{"figure": "fig:eval_montage58_fidelity", "algorithm": "heft", "calibration": "frozen-20261011", "physicalNode": "diablo03", "sequentialExecutionRequired": true}}
	request := ports.ExecutionRequest{Workflow: clone(source.Workflow), Resources: env.Resources, Runtimes: env.Runtimes, RuntimeBindings: env.RuntimeBindings, ActivityProfiles: env.Profiles, NetworkTopology: top, ExecutionScope: scope}
	rewrite := func(v any, out any) {
		b, e := json.Marshal(v)
		must(e)
		var raw any
		must(json.Unmarshal(b, &raw))
		b, e = json.Marshal(remap(raw, ids))
		must(e)
		must(json.Unmarshal(b, out))
	}
	rewrite(env, &env)
	rewrite(request, &request)
	result := []obj{}
	for _, t := range targets {
		if allowed[t["id"].(string)] {
			v := clone(t)
			v["environmentId"] = prefix
			cfg := v["configuration"].(map[string]any)
			cfg["sshSourceRanges"] = []string{ip + "/32"}
			if kind == "micro" {
				cfg["schedulingCpuCapacity"] = 1
			}
			var x obj
			rewrite(v, &x)
			result = append(result, x)
		}
	}
	// Each resource must use its environment's version, including relay and runtimes.
	for i := range env.Runtimes {
		env.Runtimes[i].EnvironmentVersionID = env.Version.ID
	}
	request.Runtimes = env.Runtimes
	return env, request, result
}
func main() {
	action := flag.String("action", "prepare", "prepare, plan or run")
	base := flag.String("api", "http://127.0.0.1:5173/akoflow-api", "daemon API")
	kind := flag.String("case", "hpc", "hpc, micro or medium")
	mode := flag.String("mode", "", "simulation or real; required for run")
	algorithm := flag.String("algorithm", "heft", "heft or moheft")
	objective := flag.String("objective", "time", "Pareto endpoint: time or cost")
	attempt := flag.Int("attempt", 1, "explicit fresh execution attempt; never resumes scientific tasks")
	campaignID := flag.String("campaign-id", "fig8-heft-20261031-home-v1", "unique campaign identifier")
	acceptQuotaEstimate := flag.Bool("accept-hpc-quota-estimate", false, "explicitly authorized per-run HPC conservative quota override; OS quota remains enforced")
	sourcePath := flag.String("source", "storage/montage-runtime-stage-calibration-20261011/medium-independent-prepared-v1/request.json", "frozen scientific execution input")
	output := flag.String("output", "outputs/heft-montage58-fig8-20261031/configuration-home", "output directory")
	ip := flag.String("ssh-ip", "", "current daemon public IPv4 address")
	workspaceRoot := flag.String("workspace-root", "", "verified shared HPC workspace root; quota remains checked")
	supplement := flag.String("transfer-profiles", "", "JSON array of evidenced missing transfer routes; existing profiles remain unchanged")
	flag.Parse()
	if *action == "signature" {
		var request, plan obj
		read(filepath.Join(*output, *kind+"-request.json"), &request)
		read(filepath.Join(*output, *kind+"-plan.json"), &plan)
		signature, e := physicalSignature(request, plan)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		save(filepath.Join(*output, *kind+"-signature.json"), obj{"signature": signature, "definition": "complete frozen inputs plus allocation, lane order and timing; excluding scheduler labels and predicted cost"})
		fmt.Println(signature)
		return
	}
	if *action == "run-all" {
		if *mode != "real" {
			fmt.Fprintln(os.Stderr, "run-all requires explicit --mode real")
			os.Exit(1)
		}
		for _, caseName := range []string{"hpc", "micro", "medium"} {
			err := campaign("run", *base, *output, caseName, *mode, *attempt, *campaignID, *acceptQuotaEstimate, *algorithm, *objective)
			_ = comparison(*base, *output, *campaignID)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		return
	}
	if *action == "report" {
		if e := comparison(*base, *output, *campaignID); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	if *action != "prepare" {
		if e := campaign(*action, *base, *output, *kind, *mode, *attempt, *campaignID, *acceptQuotaEstimate, *algorithm, *objective); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	if net.ParseIP(*ip) == nil || strings.Contains(*ip, ":") {
		panic("valid current public IPv4 required")
	}
	var source ports.ExecutionRequest
	read(*sourcePath, &source)
	if *supplement != "" {
		var extra []obj
		read(*supplement, &extra)
		model, ok := source.NetworkTopology.Metadata["workspaceMaterializationModel"].(map[string]any)
		if !ok {
			panic("source has no materialization model")
		}
		existing, _ := model["profiles"].([]any)
		key := func(v obj) string { return fmt.Sprint(v["strategy"], "/", v["sourceClass"], "/", v["targetClass"]) }
		seen := map[string]bool{}
		for _, raw := range existing {
			seen[key(raw.(map[string]any))] = true
		}
		for _, p := range extra {
			if p["source"] == nil || p["samples"] == nil {
				panic("transfer provenance and samples required")
			}
			if !seen[key(p)] {
				existing = append(existing, p)
				seen[key(p)] = true
			}
		}
		model["profiles"] = existing
		source.NetworkTopology.Metadata["supplementalTransferProfilesFile"] = *supplement
	}
	if len(source.Workflow.Activities) != 58 {
		panic("expected Montage 58")
	}
	var hpc, cloud domain.EnvironmentDefinition
	read("storage/montage-independent-20261011/hpc-clean-catalog.json", &hpc)
	read("storage/montage-independent-20261011/gcp-clean-catalog.json", &cloud)
	var targets []obj
	read("storage/montage-independent-20261011/targets-preflight.json", &targets)
	must(os.MkdirAll(*output, 0700))
	wf := requests.FromDomain(domain.WorkflowDefinition{ID: source.Workflow.WorkflowID, Name: source.Workflow.WorkflowID, Namespace: "fgcs", Version: source.Workflow})
	save(filepath.Join(*output, "workflow.json"), wf)
	for _, kind := range []string{"hpc", "micro", "medium"} {
		env, request, ts := build(source, hpc, cloud, targets, kind, *ip)
		if *workspaceRoot != "" {
			if !filepath.IsAbs(*workspaceRoot) || filepath.Clean(*workspaceRoot) == "/" {
				panic("absolute private shared workspace root required")
			}
			storageID := env.Environment.ID + "-workspace"
			connectionID := env.Environment.ID + "-plafrim-execution-ssh"
			env.Storages = append(env.Storages, domain.StorageResource{ID: storageID, EnvironmentVersionID: env.Version.ID, Name: "Figure 8 shared workspace", Type: "ssh-filesystem", Endpoint: *workspaceRoot, Shared: true, Metadata: obj{"connectionId": connectionID, "source": "verified-login-and-diablo03-20261031"}, Configuration: obj{"browseRoots": []obj{{"label": "workspace", "path": *workspaceRoot, "readOnly": false}}}})
			for i := range env.Resources {
				if env.Resources[i].Type == domain.ResourceHPCMachine {
					env.Resources[i].Metadata["workspaceStorageId"] = storageID
				}
			}
			request.Resources = env.Resources
			env.Storages[len(env.Storages)-1].RuntimeBindings = []domain.StorageRuntimeBinding{{RuntimeID: env.Environment.ID + "-plafrim-execution-slurm", HostPath: *workspaceRoot, ContainerPath: "/akoflow/data"}}
			request.ExecutionScope.Metadata["workspaceRoot"] = *workspaceRoot
			request.ExecutionScope.Metadata["storageChangedFromHistoricalHome"] = true
		}
		save(filepath.Join(*output, kind+"-environment.json"), env)
		save(filepath.Join(*output, kind+"-request.json"), request)
		save(filepath.Join(*output, kind+"-targets.json"), ts)
		fmt.Printf("%s: %d resources, %d cloud targets\n", kind, len(env.Resources), len(ts))
	}
}
