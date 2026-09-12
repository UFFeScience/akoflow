---

title: Planning, candidates, and selected plans
sidebar_label: Planning and plans
description: Why planning sessions produce candidates before one placement becomes executable.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

Planning compares where a workflow could run within a chosen execution scope. It produces predicted placements and costs so you can choose a plan before starting a run. Planning does not reserve or start infrastructure.

Use [Plan a workflow](../guides/workflows/planning) for the Desktop or API procedure; this explanation covers the model behind it.

## A planning session freezes the question

A planning session records the workflow version, available resources, network topology, deadline, budget, and selected algorithms. It also keeps the profiles and environment data used for prediction. This lets you compare candidates against the same inputs, even if the environment changes later.

<img src={useBaseUrl('/img/architecture/planning-session-lifecycle.svg')} alt="Frozen workflow and infrastructure input create a planning session. Independent algorithm runs produce candidate sets, from which one candidate becomes a schedule plan." />

## Candidates are alternatives, not executions

A candidate has a predicted makespan and cost, feasibility flags, rank and Pareto metadata, and an embedded prospective plan. Several candidates can have the same algorithm and objective. They exist so the user can inspect trade-offs before committing to one placement.

When you select a candidate, it becomes the schedule plan for the run. The plan records where activities should run and their predicted timing and cost. It can also include cloud setup actions. You can instead supply or import a placement, subject to validation.

## Objectives and constraints answer different questions

An algorithm's objective ranks candidates. Deadline and budget determine whether each candidate meets your constraints. A candidate may remain visible even when it misses a limit, so you can inspect the alternatives the planner found.

The current built-ins include HEFT, PRISM Time, and PRISM Cost. Their search and objective behavior is intentionally separate from this record model. The selected plan's prediction is also separate from observed timing: a completed run supplies the evidence needed to assess that prediction.

## Why a plan can differ from a completed run

Planning works from snapshots and models. An actual run adds provider queueing, startup behavior, data preparation, runtime availability, and observed transfer behavior. Those are recorded with the execution rather than silently rewritten into the plan. Read [plan-versus-observed evidence](./evidence-and-provenance) for how to interpret the comparison.

## Related material

- [Planning and execution state reference](../reference/planning-and-execution-states)
- [Execution scopes and topologies](../reference/execution-scopes-and-topologies)
- [Network modeling](./network-modeling)
- [Execute and monitor a workflow](../guides/workflows/executions)
