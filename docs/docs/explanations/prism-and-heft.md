---
title: "PRISM and HEFT: search, objectives, and prediction"
sidebar_label: PRISM and HEFT
description: What the built-in schedulers optimize, what each prediction includes, and why neither algorithm is guaranteed to win an observed run.
---

HEFT, PRISM Time, and PRISM Cost propose different placements for a workflow. Each produces predictions from the same planning-session inputs; none measures a completed run.

This explanation is for readers choosing or interpreting a built-in scheduler. For the procedure, see [Plan a workflow](../guides/workflows/planning). For the meaning of observed timing, see [evidence and provenance](./evidence-and-provenance).

## What is shared

All three use the session's workflow, scope, topology, profiles, deadline, and budget. They place activities only where CPU and memory fit. A multicore resource offers several lanes; an HPC partition or batch queue counts as one slot.

For each placement, a matching activity-resource profile supplies the base duration. Otherwise the planner uses `simulation.durationSeconds`, or one second if absent, then divides by resource speedup. Poor duration inputs make any scheduler comparison unreliable.

## The three schedulers

| Scheduler | Candidate search | Primary final ordering | Important modeled behavior |
|---|---|---|---|
| HEFT | One deterministic topological placement pass ordered by its upward rank | Predicted makespan, then predicted cost | Per-core availability, direct matching topology-link transfer, boot/container overhead |
| PRISM Time | Beam search over ready activities and feasible resource/core placements; complete states are re-evaluated | Predicted makespan, then transfer time, network cost, used resources, and cost | Routed transfers, active-flow sharing, resource active-window cost, optional CPU interference, queue and frozen overhead metadata |
| PRISM Cost | The same PRISM search and complete-state re-evaluation | Predicted cost, then network cost, transfer time, used resources, makespan, and queue | The same PRISM model, ranked exclusively for cost |

Later values break ties. PRISM retains some alternatives for the other objective and for network-local placements, but its bounded search cannot keep every placement.

## Ranking work before placement

HEFT ranks activities by estimated work remaining, then tests feasible resources and cores for each activity. It chooses the earliest predicted finish, using cost to break ties.

PRISM's rank also includes estimated communication over topology routes. It can explore several ready activities and placements at once. Beam width and ready-branch limit cap the search; their defaults are 120 and 3. Raising them considers more alternatives but takes more planning work. The session estimate reports expected expanded states and duration before PRISM starts.

## Why PRISM has a detailed second evaluation

PRISM ranks partial schedules quickly. For complete placements, it removes duplicates and runs a more detailed evaluation. Tasks start when their inputs and assigned lane are ready. The model includes startup overhead, optional CPU interference, and transfers along topology routes.

Overlapping flows share bandwidth at route links and resource endpoints; each link also adds latency. This predicts transfer behavior without running SimGrid during candidate generation.

The plan records the models used and a confidence value. That value reflects how many activities have a simulation definition or matching profile, not how closely a future run will match.

## Cost and makespan are different quantities

PRISM estimates resource cost from each active window and adds transfer byte cost. HEFT sums per-activity runtime cost. Both use the session's resource prices; neither is a provider invoice.

PRISM Time ranks by predicted makespan; PRISM Cost ranks by predicted cost. Beam search does not guarantee a global optimum.

## Do not infer a winner from the algorithm name

PRISM models more network and interference effects, but a richer prediction need not lead to a faster run. Durations, queues, storage paths, startup, and transfers can differ from the session's inputs. Either scheduler may perform better for a particular workflow and scope.

HEFT and PRISM do **not** pass through one shared evaluator before their predictions are compared. Treat their estimates as algorithm-specific. To compare outcomes, run selected plans under comparable conditions and inspect the observed traces.

## A disciplined comparison

1. Use the same workflow version, scope, topology, activity profiles, deadline, and budget for every algorithm in the session.
2. Inspect assignments, predicted transfers, and cost before selecting a plan.
3. Run the selected alternatives under comparable runtime conditions.
4. Compare observed makespan, task timing, transfers, and cost with the plan.
5. Update profiles or infrastructure values when a recurring prediction gap has a known cause.

## Related material

- [Planning, candidates, and selected plans](./planning)
- [Network modeling and data movement](./network-modeling)
- [Execution scopes and topologies reference](../reference/execution-scopes-and-topologies)
- [Planning and execution state reference](../reference/planning-and-execution-states)
