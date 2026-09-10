---
title: Execute and monitor a workflow
---

# Execute and monitor a workflow

An execution run applies one immutable schedule plan to the workflow and infrastructure snapshots supplied in its request. Real and simulated runs share the same run, activity, timing, transfer, and cost model, which makes planned-versus-observed comparison possible.

## Modes and run types

- **Real** runs dispatch activities through execution runtimes such as a Kubernetes or SLURM adapter configured by the environment.
- **Simulation** runs dispatch simulation-capable activities through a simulation runtime such as SimGrid.
- **Interactive** sessions open a terminal against a compatible resource. They are represented in the unified run history, but are opened through the console-session API rather than the planned workflow execution request.

The run history distinguishes `workflow`, `interactive`, and `standalone` kinds.

## Status and timing

A workflow run moves through `created`, `running`, and either `completed` or `failed`. Its activities expose the more detailed states `blocked`, `ready`, `preparing`, `running`, `completed`, `failed`, and `cancelled`.

Runtime handles distinguish `starting`, `running`, `completed`, `failed`, and `stopped`. For real runtimes, submitted time means the control plane handed work to the runtime; started time means the runtime allocated it; container-started time marks when user code could begin inside the container.

The completed trace includes:

- makespan and total cost;
- compute, transfer, queue, interference, and overhead time;
- activity placement and runtime handles;
- transferred bytes, transfer duration/cost, strategy, and route;
- observed task intervals alongside predicted assignments.

## Using AkôFlow Desktop

1. Open a selected or manually created schedule plan.
2. Choose the action to run it.
3. Confirm the generated run ID and review the predicted allocation timeline. The mode is derived from the plan's execution scope: simulation scopes produce simulation runs; execution scopes produce real runs.
4. Choose **Start execution**.
5. Open **Runs** to follow the unified history. Filter by type, mode, or status.
6. Open the run to inspect live activity state, logs, placement, network transfers, and execution prerequisites.
7. After completion, compare predicted and observed Gantt charts, per-activity runtime gaps, makespan, cost, and the execution-time decomposition.

<!-- screenshot: Start execution form with plan selector, derived mode, compatible runtimes, and predicted Gantt annotated -->

<!-- screenshot: Runs list with Type, Mode, and Status filters annotated -->

<!-- screenshot: Live workflow monitor with activity states, selected activity logs, placement, and transfer network annotated -->

<!-- screenshot: Completed run comparison showing planned and observed Gantts, runtime gaps, cost, and time decomposition -->

To open an interactive terminal, use the console action for a compatible resource. The session appears with interactive runs in **Runs** and can be closed or have its log exported.

<!-- screenshot: Interactive terminal opened for a resource, with connection state and close/export controls annotated -->

## Using the API

`POST /execution-runs/` accepts a complete, reproducible execution envelope. The checked-in files `examples/simulation/execution-request.yaml` and `examples/kind/requests/execution-request.yaml` are canonical examples for simulation and real Kubernetes execution respectively.

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/simulation/execution-request.yaml \
  "$AKOFLOW_API_URL/execution-runs/"
```

The request contains `run`, `plan`, `workflow`, `executionScope`, `resources`, `runtimes`, runtime bindings, the network topology, and activity profiles. Submission is asynchronous and returns `202 Accepted` with the queued job. Read the run by the `run.id` in the request:

```bash
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/execution-runs/simulation-example-run-v1/"
```

The detail response contains `run`, `activities`, `dataTransfers`, `handles`, and `events`; it can also include related infrastructure operations. List endpoints support the Desktop's run history and filters:

```bash
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/execution-runs/"
```

Interactive terminals use the console endpoints:

```bash
# Inspect available console commands and their required arguments
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/console-commands/"

# After opening a session, stream it with:
# GET /console-sessions/<session-id>/stream/
# Close it with:
# DELETE /console-sessions/<session-id>/
# Export its log with:
# GET /console-sessions/<session-id>/log/
```

Consult `GET /console-commands/` before constructing an open-session request because compatibility and arguments depend on the resources and runtimes registered in the instance.

## Investigating a failure

Start with `run.failureReason`, then inspect the failed activity, its handle `failure`, exit code and log, and the ordered run events. If the activity remained in `preparing`, inspect executable/workspace preparation and transfer records before the runtime log.

