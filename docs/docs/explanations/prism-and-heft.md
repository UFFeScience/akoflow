---
title: "PRISM and HEFT: search, objectives, and prediction"
sidebar_label: PRISM and HEFT
description: What the built-in schedulers optimize, what each prediction includes, and why neither algorithm is guaranteed to win an observed run.
---

HEFT, PRISM Time, and PRISM Cost are alternatives for generating schedule-plan candidates. They are not measurements of a completed execution. Their output is useful only in the context of the frozen workflow, resources, topology, profiles, deadline, and budget of one planning session.

This explanation is for readers choosing or interpreting a built-in scheduler. For the procedure, see [Plan a workflow](../guides/workflows/planning). For the meaning of observed timing, see [evidence and provenance](./evidence-and-provenance).

## What is shared

All three schedulers receive the same `PlanningRequest`: a workflow version, execution scope, schedulable resources, network topology, activity-resource profiles, deadline, budget, and optional interference matrix. They reject a scope with no schedulable resources and only place an activity on a resource that satisfies its CPU and memory requirements. A resource with multiple cores offers multiple scheduling lanes, except for an opaque batch target such as an HPC partition or batch queue, which is treated as one slot.

The base duration for a placement is selected from an activity-resource profile when one matches. Otherwise the planner uses the activity's `simulation.durationSeconds` when present, falling back to one second and then dividing by the resource's compute speedup. These inputs need to be credible before any comparison of algorithm quality is meaningful.

## The three schedulers

| Scheduler | Candidate search | Primary final ordering | Important modeled behavior |
|---|---|---|---|
| HEFT | One deterministic topological placement pass ordered by its upward rank | Predicted makespan, then predicted cost | Per-core availability, direct matching topology-link transfer, boot/container overhead |
| PRISM Time | Beam search over ready activities and feasible resource/core placements; complete states are re-evaluated | Predicted makespan, then transfer time, network cost, used resources, and cost | Routed transfers, active-flow sharing, resource active-window cost, optional CPU interference, queue and frozen overhead metadata |
| PRISM Cost | The same PRISM search and complete-state re-evaluation | Predicted cost, then network cost, transfer time, used resources, makespan, and queue | The same PRISM model, ranked exclusively for cost |

"Primary" does not mean that later values are ignored. They are deterministic tie-breakers. PRISM also reserves parts of each beam for the other objective and for network-local placements. In a time search, one lane keeps alternatives by concrete earliest finish; this reduces the chance that a tie in the projected critical path discards a low-wait placement too early. It is search diversity, not a promise that every possible placement is retained.

## Ranking work before placement

HEFT computes an upward rank from average activity duration and the longest successor rank. It then considers every feasible resource and every core for the next ranked activity, choosing the earliest resulting finish; equal makespans are broken by predicted cost.

PRISM also constructs a rank, but its rank includes average communication time over routes in the frozen topology. Its ready frontier can branch to several ready activities, and each partial state can place the chosen activity on each feasible resource. The beam width and ready-branch limit bound that exploration; the registered defaults are 120 and 3. Larger values can retain more alternatives but increase planning work. The planning-session estimate reports the expected expanded states and calibrated duration before a PRISM run starts.

## Why PRISM has a detailed second evaluation

The compact PRISM search needs to rank partial schedules quickly. After it reaches complete placement states, it removes duplicate placement signatures and re-evaluates each retained state with an event model. The evaluator starts tasks when their inputs and assigned lane are ready, applies frozen boot and container overhead, models active tasks on a resource with optional pairwise CPU-priority interference, and progresses transfer flows along the cached topology routes.

For a network flow, the evaluator counts active users of each route hop as well as active flows sharing the sending or receiving resource. Bandwidth is shared among the relevant flows, and link latency is paid before payload movement. This is a prediction model, not an invocation of a real SimGrid process while the candidate is being generated.

The resulting PRISM plan records evaluator metadata including its prediction confidence, network-path model, network-contention model, active-window cost model, and interference model. Prediction confidence reflects the fraction of activities with a simulation definition or matching activity profile; it does not establish that a future run will match the prediction.

## Cost and makespan are different quantities

PRISM charges a resource by its active window in the detailed evaluation, then adds modeled transfer byte price. This can differ from HEFT's accumulated per-activity runtime price. Both are estimates derived from the frozen resource prices and assigned placement; neither is an invoice from a cloud provider.

PRISM Time ranks candidates by predicted makespan. PRISM Cost ranks by predicted cost. Neither objective asserts that the candidate is globally optimal, because beam search intentionally bounds the set of partial schedules that survive.

## Do not infer a winner from the algorithm name

PRISM has a richer current network and interference model, but more modeled inputs do not guarantee a better observed run. A real or simulated execution can differ when activity durations, topology, provider queueing, storage paths, startup overhead, or actual transfer behavior differ from the frozen inputs. HEFT can therefore have a lower observed makespan for a particular scope and workflow. Conversely, PRISM can find a better plan when its additional modeled effects distinguish placements that HEFT treats similarly.

The current implementation does **not** send HEFT and PRISM candidates through one shared post-search evaluator before ranking across algorithms. Compare their stored predictions as algorithm-specific estimates, then execute selected plans under the same scope and inspect their observed traces. The evidence, rather than the algorithm label, establishes which plan performed better in that experiment.

## A disciplined comparison

1. Use the same workflow version, scope, topology, activity profiles, deadline, and budget for every algorithm in the session.
2. Inspect candidate assignments and predicted transfer/cost fields before selecting a plan; different placements may explain different outcomes.
3. Run the selected alternatives under comparable runtime conditions.
4. Compare observed makespan, task timing, transfers, and cost with the plan.
5. Calibrate the workflow profiles or infrastructure model when a recurring prediction gap has a concrete cause; do not treat a single result as proof of algorithm superiority.

## Related material

- [Planning, candidates, and selected plans](./planning)
- [Network modeling and data movement](./network-modeling)
- [Execution scopes and topologies reference](../reference/execution-scopes-and-topologies)
- [Planning and execution state reference](../reference/planning-and-execution-states)
