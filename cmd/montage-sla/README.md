# Montage 6448 SLA campaign (AKO-213)

This command runs only local simulation. It never contacts the daemon, provisions
cloud infrastructure or launches a scientific task on an external machine.
Run inside the development container, where the native SimGrid runner is installed:

```sh
go run ./cmd/montage-sla --action all
```

The current output root defaults to `outputs/montage6448-sla-20261101-v2`.
The provisional v1 cohort is preserved but excluded: it exposed a preexisting
map-order tie in PRISM. Canonical signature ordering now resolves tied candidates
identically in both conditions, without changing their objective scores.

After completion, run `--action verify` in the same container. This reads back
all 287 native traces (seven references plus 280 scenario replays), rather than
merely counting output files. Run `--action render` on the host to generate the
two PGFPlots sources and paired ablation table from the version 2 audit; compile
those sources with the local LaTeX toolchain.

The workflow, seven S1–S7 paper environments, profiles, directed network,
resource prices and overheads are frozen from the prior paper request snapshots.
All seven workflow hashes must match. Planner/model source hashes and the native
SimGrid binary hash are fixed in the manifest; incompatible resumes fail closed.
A completed row is reusable only after its input, model, plan and trace hashes
and full-activity completion are verified. Source observations are not overwritten.

Fresh unconstrained native HEFT plans are simulated first in every environment.
Let T_H and C_H be their simulated makespan and cost. B_0=C_H when positive;
otherwise B_0 is the positive fresh S4 HEFT cost. Zero-cost HPC scopes remain
in the seven-environment aggregate and are explicitly labeled non-informative
for relative monetary discrimination. No resource price is invented or changed.

Margins are 0.8,0.9,1.0,1.1,1.2. In the time panel D=margin*T_H and B=B_0;
in the cost panel D=T_H and B=margin*B_0. Native HEFT is the same baseline
mapping, assessed against each limit. The main comparison has210 rows:
7 environments *5 margins *2 panels *3 methods (PRISM,HEFT,MOHEFT).

PRISM uses beamWidth20,optionCount25,readyBranchLimit3 and interferenceAware=true
in every condition. SLA-on uses original production pruning and final filtering.
SLA-off disables only those two constraint-guided operations, preserving emitted
original limits and predicted feasibility. The off search is unconstrained and
is reusable across margins; every scenario still receives its own native replay.
The ablation adds70 off rows, paired with the70 on rows already in the main set.

MOHEFT uses its actual compiled Go plugin, with K=25. Its native search is
unconstrained. Candidate selection uses only native predicted makespan and
paper task-additive cost: select the fastest/cheapest predicted-feasible Pareto
candidate for the requested objective. If none is predicted-feasible, retain the
unconstrained objective endpoint as explicitly infeasible best effort. No native
simulation is used to pick an MOHEFT candidate, and no PRISM operator is injected.
Every candidate's scores and hash are retained before its selected-plan replay.

All methods are evaluated with the same native SimGrid ProcessExecutor, including
resource-lane order, shared-network events and common VM lifecycle compute/disk
billing. Original native predicted costs are preserved and identified; they are
not silently converted into another scheduler's optimization objective. Report
predicted feasibility separately from actual simulated deadline,budget and joint
SLA compliance. A violation never terminates the simulation early or counts as a
missing case. Compare with a fixed1e-9 relative/absolute numerical tolerance.

Every native replay must complete6448 activities and pass exact resource,
assignment,workflow-precedence and lane-precedence checks. The figure denominator
is seven environments,not stochastic repetitions. Main plots are descriptive
sensitivity curves,not confidence intervals. No simulation outcome refits the
frozen models or thresholds. Existing paper data and results remain separate.
