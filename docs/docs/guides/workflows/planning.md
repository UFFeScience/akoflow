---
title: Plan a workflow
---

# Plan a workflow

Plan a workflow to compare possible placements before starting a run. A planning session uses one workflow version and one execution scope, then keeps the candidates produced by the selected algorithms.

Select a candidate to save it as the schedule plan for execution.

For the API commands on this page, complete [API connection setup](../../tutorials/api-access) first.

## Sessions, algorithms, and candidates

A session records its workflow version, execution scope, network topology, selected algorithms, optional deadline and budget, progress, and final selection. Its status is `queued`, `running`, `completed`, `failed`, or `cancelled`.

The Desktop currently offers PRISM time and cost objectives plus HEFT when those algorithms are returned by the server. Always use `GET /planning-algorithms/` as the authoritative list for an installed instance. PRISM accepts an option count and beam width; an optional directed interference matrix can also be attached to the session. HEFT does not use that matrix while planning.

Each candidate reports:

- algorithm, objective, and rank;
- Pareto-optimal/dominated and feasible flags;
- predicted makespan and cost;
- a complete schedule with activity-to-resource assignments.

Assignments include predicted ready, start, finish, runtime, transfer time, cost, core/slot placement, and order on the resource. Plans can also contain infrastructure lifecycle actions when cloud capacity is involved.

## Using AkôFlow Desktop

1. Open a workflow definition and choose **Generate plan**. Planning sessions are created from that workflow so the session stays bound to its immutable version.
2. Choose **Automatic planning**.
3. Select an execution or simulation scope and its network topology.
4. Select one or more algorithms. Configure PRISM search options if applicable.
5. Optionally set a deadline, budget, or import an interference matrix.
6. Choose **Generate candidate plans**.
7. Follow each algorithm run's progress. Expand candidates to inspect their Gantt timelines and assignments.
8. Compare predicted time, cost, feasibility, and Pareto status, then select a candidate. AkôFlow creates the executable schedule plan from that candidate.

<img src={require('@site/static/img/interface/planning/create-execution-plan.png').default} alt="AkôFlow Desktop Create an execution plan screen in light mode, with generated and manual planning choices, a planning target selector, an execution scope, and PRISM Cost, PRISM Time, and HEFT controls." />

*Choose **Generate plans** to compare candidate schedules. The target selector keeps real execution scopes separate from simulation-only scopes; PRISM Cost and PRISM Time are exclusive objectives, while HEFT is a comparison baseline.*

### Manual plans

Choose the manual planning mode when placement is known in advance. Select a scope and topology, then assign every activity to a compatible runtime and resource and provide its expected duration. The Desktop computes a dependency-aware schedule and submits the complete plan for validation.

### Imported plans

The plans API also accepts a complete plan as imported data. Imported IDs must refer to an existing workflow version, execution scope, topology, and resources; the server validates the plan before saving it.

## Using the API

First discover the algorithms available in the running instance:

```bash
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/planning-algorithms/"
```

Create an automatic planning session. The IDs below refer to the environment, scope, topology, and workflow registered in the [SimGrid first-run sequence](./first-run). Complete those registration steps first, or replace all four IDs with records from your own instance:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "planning-simulation-example",
    "workflowVersionId": "simulation-example-workflow-v1",
    "executionScopeId": "simulation-example-v1-scope",
    "networkTopologyId": "simulation-network-v1",
    "algorithms": [
      {"id": "prism-time", "configuration": {"optionCount": 25, "beamWidth": 120}},
      {"id": "prism-cost", "configuration": {"optionCount": 25, "beamWidth": 120}},
      {"id": "heft", "configuration": {}}
    ],
    "deadlineSeconds": 0,
    "budget": 0
  }' \
  "$AKOFLOW_API_URL/planning-sessions/"
```

Creation returns `202 Accepted`. Poll the session and list its candidates:

```bash
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/planning-sessions/planning-simulation-example/"

curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/planning-sessions/planning-simulation-example/candidates/"
```

Read a candidate before selecting it, then promote it to a plan:

```bash
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/planning-sessions/planning-simulation-example/candidates/<candidate-id>/"

curl --fail-with-body -X POST \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/planning-sessions/planning-simulation-example/candidates/<candidate-id>/select/"
```

Selection returns `201 Created` with the saved schedule plan.

For a manual plan, send the complete validation envelope used by `examples/simulation/plan-request.yaml` to `POST /schedule-plans/`. To import an already assembled plan whose referenced objects are registered, send `{ "plan": ... }` to `POST /schedule-plans/import/`; the server sets its source to `imported` and validates it.

## Next step

Review the selected plan and [start and monitor an execution](./executions.md).
