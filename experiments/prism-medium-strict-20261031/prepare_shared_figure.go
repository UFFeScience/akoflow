//go:build ignore

// Keep representative Figure8 observations separate from independent replays.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string) []map[string]any {
	b, e := os.ReadFile(p)
	must(e)
	var rows []map[string]any
	must(json.Unmarshal(b, &rows))
	if len(rows) != 6 {
		panic("six observations required")
	}
	return rows
}
func write(p string, b []byte) {
	must(os.MkdirAll(filepath.Dir(p), 0755))
	must(os.WriteFile(p, b, 0600))
}
func main() {
	if len(os.Args) != 4 {
		panic("SHARED_EXPORT_ROOT INDEPENDENT_EXPORT_ROOT PAPER_ROOT")
	}
	root, independent, paper := os.Args[1], os.Args[2], os.Args[3]
	rows := read(root + "/figure8-observations.json")
	own := read(independent + "/figure8-observations.json")
	byCase := map[string]map[string]any{}
	ownIDs := map[string]bool{}
	counts := map[string]int{}
	plottedByCase := map[string]map[string]any{}
	for _, r := range own {
		id := r["real_run_id"].(string)
		if ownIDs[id] {
			panic("independent observations must be distinct")
		}
		ownIDs[id] = true
		byCase[r["algorithm"].(string)+"-"+r["environment"].(string)] = r
	}
	for _, r := range rows {
		counts[r["real_run_id"].(string)]++
		plottedByCase[r["algorithm"].(string)+"-"+r["environment"].(string)] = r
	}
	if len(counts) != 4 {
		panic("four representative physical observations required")
	}
	for _, pair := range [][2]string{{"heft-hpc", "moheft-hpc"}, {"prism-time-medium", "heft-medium"}} {
		first, second := plottedByCase[pair[0]], plottedByCase[pair[1]]
		if first == nil || second == nil || first["real_run_id"] != second["real_run_id"] || counts[first["real_run_id"].(string)] != 2 {
			panic("expected audited shared pair missing")
		}
	}
	for _, r := range rows {
		k := r["algorithm"].(string) + "-" + r["environment"].(string)
		previous, exists := byCase[k]
		if !exists {
			panic("missing independent case")
		}
		for _, f := range []string{"planned_makespan_seconds", "simulated_makespan_seconds", "plan_id", "input_sha256", "plan_sha256", "native_trace_sha256"} {
			if r[f] != previous[f] {
				panic("frozen forecast changed " + k + " " + f)
			}
		}
		shared := counts[r["real_run_id"].(string)] == 2
		r["shared_real_observation"] = shared
		r["figure_observation_basis"] = "individual selected-plan execution"
		if shared {
			r["figure_observation_basis"] = "shared representative execution of audited equivalent schedule"
			if r["environment"] == "hpc" {
				r["shared_real_group"] = "heft-hpc"
			} else {
				r["shared_real_group"] = "prism-time-medium"
			}
		}
		r["independent_real_run_id"] = previous["real_run_id"]
		r["independent_real_makespan_seconds"] = previous["real_makespan_seconds"]
		r["cohort"] = "Figure8 representative observations for equivalent schedules; independent executions preserved separately"
	}
	b, e := json.MarshalIndent(rows, "", "  ")
	must(e)
	write(root+"/figure8-observations.json", b)
	f, e := os.Open(root + "/figure8-observations.csv")
	must(e)
	table, e := csv.NewReader(f).ReadAll()
	must(e)
	must(f.Close())
	columns := table[0]
	for _, column := range []string{"shared_real_observation", "figure_observation_basis", "independent_real_run_id", "independent_real_makespan_seconds"} {
		exists := false
		for _, previous := range columns {
			if previous == column {
				exists = true
			}
		}
		if !exists {
			columns = append(columns, column)
		}
	}
	out, e := os.Create(root + "/figure8-observations.csv")
	must(e)
	w := csv.NewWriter(out)
	must(w.Write(columns))
	for _, r := range rows {
		line := []string{}
		for _, k := range columns {
			v := r[k]
			s := ""
			if n, ok := v.(float64); ok {
				s = strconv.FormatFloat(n, 'f', 9, 64)
			} else if v != nil {
				s = fmt.Sprint(v)
			}
			line = append(line, s)
		}
		must(w.Write(line))
	}
	w.Flush()
	must(w.Error())
	must(out.Close())
	target := "experiments/prism-medium-strict-20261031/figure8-shared-summary"
	for _, ext := range []string{"csv", "json"} {
		b, e = os.ReadFile(root + "/figure8-observations." + ext)
		must(e)
		write(target+"/figure8-observations."+ext, b)
		write(paper+"/figs/data/montage58-time-all-recalibrated."+ext, b)
		b, e = os.ReadFile(independent + "/figure8-observations." + ext)
		must(e)
		write(target+"/independent-executions."+ext, b)
		write(paper+"/figs/data/montage58-time-independent-executions."+ext, b)
	}
	write(target+"/README.md", []byte("# Figure8: shared representative observations\n\nFour physical executions represent six scheduler/environment cases. HPC HEFT and MOHEFT-TIME use the audited HEFT observation1608.7642674446106s. HPC+medium PRISM-TIME and HEFT use the audited PRISM observation3353.9606051445007s. These are shared observations of equivalent schedules, not two independent measurements with identical durations. Asterisks identify each pair within its panel.\n\nIndependent replays remain unchanged in independent-executions.csv/json: MOHEFT-TIME/HPC1261.642051935196s and HEFT/HPC+medium3173.3246052265167s. The other four independent observations are retained too. All forecasts and frozen input hashes are unchanged. Repeated measurements illustrate run-to-run variation; they do not isolate its cause or establish scheduling superiority.\n\nThe plotted CSV/JSON retain actual real_run_id, executed_plan_id and trace/request hashes, and explicitly label shared_real_observation, shared_real_group and figure_observation_basis. They also link each independent execution's ID and duration. No source observation was overwritten. Error and range values correspond to the representative real used by the figure.\n"))
	fmt.Println("Prepared four explicitly identified representative observations and preserved six independent observations")
}
