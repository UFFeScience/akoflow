package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func renderPlots(root string) {
	var coverage struct {
		AuditVersion  int  `json:"audit_version"`
		Complete      bool `json:"complete"`
		Main          int  `json:"main_cases"`
		Off           int  `json:"additional_sla_off_cases"`
		NativeChecked bool `json:"all_assignments_and_precedences_checked"`
	}
	if !load(filepath.Join(root, "coverage-audit.json"), &coverage) || coverage.AuditVersion < 2 || !coverage.Complete || coverage.Main != 210 || coverage.Off != 70 || !coverage.NativeChecked {
		panic("complete version2 scientific audit required before rendering")
	}
	var summary []aggregate
	var paired []ablation
	if !load(filepath.Join(root, "summary.json"), &summary) || !load(filepath.Join(root, "ablation-summary.json"), &paired) || len(summary) != 30 || len(paired) != 10 {
		panic("complete figure data missing")
	}
	index := map[string]aggregate{}
	for _, a := range summary {
		if a.N != 7 {
			panic("incorrect figure denominator")
		}
		index[fmt.Sprintf("%s/%s/%.1f", a.Panel, a.Algorithm, a.Margin)] = a
	}
	dir := filepath.Join(root, "figures")
	must(os.MkdirAll(dir, 0700))
	for _, panel := range []string{"time", "cost"} {
		var tex strings.Builder
		axisLabel := "Deadline margin relative to HEFT"
		title := "(a) Deadline sensitivity"
		if panel == "cost" {
			axisLabel = "Budget margin relative to HEFT"
			title = "(b) Budget sensitivity"
		}
		tex.WriteString("\\documentclass[tikz,border=4pt]{standalone}\n\\usepackage{pgfplots}\n\\usetikzlibrary{patterns}\n\\pgfplotsset{compat=1.18}\n\\begin{document}\n\\begin{tikzpicture}\n\\begin{axis}[width=11cm,height=7.2cm,ybar,bar width=9pt,ymin=0,ymax=112,xmin=-0.65,xmax=4.65,xtick={0,1,2,3,4},xticklabels={$-20\\%$,$-10\\%$,$0\\%$,$+10\\%$,$+20\\%$},ytick={0,25,50,75,100},yticklabels={$0\\%$,$25\\%$,$50\\%$,$75\\%$,$100\\%$},axis lines*=left,ymajorgrids=true,grid style={gray!25},tick label style={font=\\small},label style={font=\\small},title style={font=\\normalsize},legend style={at={(0.5,1.09)},anchor=south,legend columns=3,draw=none,font=\\footnotesize,/tikz/every even column/.append style={column sep=6pt}},nodes near coords,point meta=explicit symbolic,every node near coord/.append style={font=\\scriptsize,black},clip=false,\n")
		fmt.Fprintf(&tex, "xlabel={%s},ylabel={Joint SLA met (7 environments)},title={%s}]\n", axisLabel, title)
		for i, algorithm := range []string{"heft", "prism-" + panel, "moheft-" + panel} {
			style := []string{"draw=black,fill=gray!55", "draw=black,fill=black", "draw=black,fill=white,postaction={pattern=north east lines,pattern color=black}"}[i]
			fmt.Fprintf(&tex, "\\addplot+[%s] coordinates {", style)
			for j, margin := range margins {
				a, exists := index[fmt.Sprintf("%s/%s/%.1f", panel, algorithm, margin)]
				if !exists {
					panic("missing graph point")
				}
				fmt.Fprintf(&tex, "(%d,%.12f)[%d] ", j, a.Percent, a.Met)
			}
			tex.WriteString("};\n")
		}
		name := "TIME"
		if panel == "cost" {
			name = "COST"
		}
		fmt.Fprintf(&tex, "\\legend{HEFT,PRISM-%s,MOHEFT-%s}\n\\end{axis}\n\\end{tikzpicture}\n\\end{document}\n", name, name)
		must(os.WriteFile(filepath.Join(dir, "fig-sla-montage6448-"+panel+".tex"), []byte(tex.String()), 0600))
	}
	var table strings.Builder
	table.WriteString("% Generated from the audited paired PRISM ablation; all values are cases out of7.\n\\begin{table}[!t]\n\\centering\n\\color{magenta}\n\\small\n\\caption{PRISM SLA ablation on Montage6448. Each cell reports SLA-on / SLA-off cases meeting both limits, out of seven environments. All other search settings and simulation inputs are identical.}\n\\label{tab:montage_sla_ablation}\n\\begin{tabular}{lrr}\n\\toprule\nMargin & Time (on/off) & Cost (on/off) \\\\\n\\midrule\n")
	for _, margin := range margins {
		var a, b ablation
		for _, p := range paired {
			if p.Margin == margin {
				if p.Panel == "time" {
					a = p
				} else {
					b = p
				}
			}
		}
		if a.N != 7 || b.N != 7 {
			panic("missing paired table row")
		}
		fmt.Fprintf(&table, "$%+.0f\\%%$ & %d / %d & %d / %d \\\\\n", 100*(margin-1), a.OnMet, a.OffMet, b.OnMet, b.OffMet)
	}
	table.WriteString("\\bottomrule\n\\end{tabular}\n\\end{table}\n")
	must(os.WriteFile(filepath.Join(dir, "montage6448-sla-ablation-table.tex"), []byte(table.String()), 0600))
	renderPaperSection(dir, summary, paired)
	logf("generated two monochrome graph sources and paired ablation table from complete audited results")
}

// Derive the prose from the same audited aggregates as the bars and table.
func renderPaperSection(dir string, summary []aggregate, paired []ablation) {
	totals := map[string]aggregate{}
	for _, a := range summary {
		key := a.Panel + "/" + a.Algorithm
		t := totals[key]
		t.N += a.N
		t.Met += a.Met
		t.MonetaryN += a.MonetaryN
		t.MonetaryMet += a.MonetaryMet
		t.FalseFeasible += a.FalseFeasible
		totals[key] = t
	}
	var tex strings.Builder
	tex.WriteString(`% Generated from complete audited Montage 6448 SLA results.
\textcolor{magenta}{Figure~\ref{fig:eval_sla} separates deadline and budget sensitivity for Montage 6448.
Each bar counts environments meeting both limits, including unsuccessful best-effort schedules in the denominator.
The limits are local to each environment's fresh HEFT reference; these results replace the earlier cross-workflow SLA aggregate.}

\begin{figure*}[pos=!t]
\centering
\includegraphics[width=0.49\textwidth]{figs/fig-sla-montage6448-time.pdf}\hfill
\includegraphics[width=0.49\textwidth]{figs/fig-sla-montage6448-cost.pdf}
\caption{\textcolor{magenta}{Joint SLA compliance for Montage 6448 in S1--S7 (counts above bars, out of seven).
(a) Deadline $D=\alpha T_H$, with budget $B_0$ fixed: PRISM-TIME, HEFT, and MOHEFT-TIME.
(b) Budget $B=\alpha B_0$, with deadline $T_H$ fixed: PRISM-COST, HEFT, and MOHEFT-COST.
Margins are relative to each environment's simulated HEFT reference.
S1 and S2 have no charged resources and use S4's positive HEFT cost as budget reference; their deadline guard still applies.
These are deterministic environment counts, not repeated-run success probabilities.}}
\label{fig:eval_sla}
\end{figure*}

`)
	for _, panel := range []string{"time", "cost"} {
		p, h, m := totals[panel+"/prism-"+panel], totals[panel+"/heft"], totals[panel+"/moheft-"+panel]
		if p.N != 35 || h.N != 35 || m.N != 35 || p.MonetaryN != 25 {
			panic("incorrect paper totals")
		}
		fmt.Fprintf(&tex, "\\textcolor{magenta}{Across the 35 environment--margin conditions in the %s comparison, PRISM-%s meets both limits in %d cases, HEFT in %d, and MOHEFT-%s in %d.\nWithin the five environments with positive HEFT charges (25 conditions), the corresponding counts are %d, %d, and %d.}\n\n", panel, strings.ToUpper(panel), p.Met, h.Met, strings.ToUpper(panel), m.Met, p.MonetaryMet, h.MonetaryMet, m.MonetaryMet)
	}
	tex.WriteString(`\textcolor{magenta}{HEFT's deadline curve is determined by construction: its reference schedule cannot meet a deadline below its own simulated makespan and meets it at zero or positive margin.
The two zero-charge cluster environments cannot discriminate relative monetary savings, so their contribution to the budget panel must be read with the fixed deadline guard.
The positive-charge subset is reported separately rather than assigning synthetic cluster prices.}

\input{sections/montage6448-sla-ablation-table}

`)
	for _, panel := range []string{"time", "cost"} {
		var a ablation
		for _, p := range paired {
			if p.Panel == panel {
				a.N += p.N
				a.OnOnly += p.OnOnly
				a.OffOnly += p.OffOnly
				a.Both += p.Both
				a.Neither += p.Neither
				a.DifferentSchedules += p.DifferentSchedules
			}
		}
		if a.N != 35 {
			panic("incorrect paired paper totals")
		}
		fmt.Fprintf(&tex, "\\textcolor{magenta}{For PRISM-%s, the paired SLA ablation changes the selected schedule in %d of 35 conditions. SLA-on alone meets both limits in %d conditions, SLA-off alone in %d, both in %d, and neither in %d (Table~\\ref{tab:montage_sla_ablation}).}\n\n", strings.ToUpper(panel), a.DifferentSchedules, a.OnOnly, a.OffOnly, a.Both, a.Neither)
	}
	tex.WriteString(`\textcolor{magenta}{The ablation isolates constraint-guided search and selection within the retained candidate set; it is not a claim of global feasibility or optimality.
Native predicted feasibility can disagree with replayed compliance, including for MOHEFT's task-additive cost forecast versus common VM-lifecycle billing.
The outcome therefore measures whether the selected schedule meets the limits in the shared simulator, not whether its planner declares it feasible.
The scope is one workflow and seven frozen environment models; the counts do not establish statistical superiority across workflows or validate real execution.}
`)
	must(os.WriteFile(filepath.Join(dir, "montage6448-sla-results.tex"), []byte(tex.String()), 0600))
}
