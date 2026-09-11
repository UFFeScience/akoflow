---
title: Planning, candidates, and selected plans
sidebar_label: Planning and plans
description: Why planning sessions produce candidates before one placement becomes executable.
---

Planning answers a bounded placement question: given one workflow version and one execution scope, which feasible assignment should be used for the chosen objective? It is not execution, and it does not reserve or start infrastructure.

Use [Plan a workflow](../guides/workflows/planning) for the Desktop or API procedure; this explanation covers the model behind it.

## A planning session freezes the question

The session stores the workflow version, execution scope, network topology, environment snapshots, resources, activity profiles, deadline, budget, interference data, and algorithm selection. Freezing these inputs makes a later comparison meaningful: each algorithm evaluates the same recorded infrastructure universe instead of whatever discovery happens to return later.

```text
workflow version + scope + snapshots + constraints
                         |
                         v
                  planning session
                         |
          +--------------+--------------+
          |                             |
     algorithm run                  algorithm run
          |                             |
      candidate(s)                 candidate(s)
          \                             /
           \---- select one candidate -/
                         |
                    schedule plan
```

## Candidates are alternatives, not executions

A candidate has a predicted makespan and cost, feasibility flags, rank and Pareto metadata, and an embedded prospective plan. Several candidates can have the same algorithm and objective. They exist so the user can inspect trade-offs before committing to one placement.

Only selection promotes a candidate to the canonical schedule plan. The plan contains assignments to a resource/core/slot and prediction fields such as ready, start, finish, runtime, transfer, and cost. It may also contain cloud lifecycle actions. A manually authored placement and an imported placement use the same schedule-plan representation after validation.

## Objectives and constraints answer different questions

An algorithm objective ranks candidates, while deadline and budget determine whether a candidate is feasible for the request. A candidate can remain visible when it misses a constraint; that preserves the best alternatives discovered even if no candidate meets the SLA. It is not a promise that the option will be recommended for execution.

The current built-ins include HEFT, PRISM Time, and PRISM Cost. Their search and objective behavior is intentionally separate from this record model. The selected plan's prediction is also separate from observed timing: a completed run supplies the evidence needed to assess that prediction.

## Why a plan can differ from a completed run

Planning works from snapshots and models. An actual run adds provider queueing, startup behavior, data preparation, runtime availability, and observed transfer behavior. Those are recorded with the execution rather than silently rewritten into the plan. Read [plan-versus-observed evidence](./evidence-and-provenance) for how to interpret the comparison.

## Related material

- [Planning and execution state reference](../reference/planning-and-execution-states)
- [Execution scopes and topologies](../reference/execution-scopes-and-topologies)
- [Network modeling](./network-modeling)
- [Execute and monitor a workflow](../guides/workflows/executions)
