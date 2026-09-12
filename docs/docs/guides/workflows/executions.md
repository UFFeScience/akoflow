---
title: Execute and monitor a workflow
---

# Execute and monitor a workflow

Start a run from a saved plan, then follow its activities and inspect the result. AkôFlow records both real and simulated runs with timing, transfer, and cost information so you can compare what happened with the plan's predictions.

For the API commands on this page, complete [API connection setup](../../tutorials/api-access) first.

## Modes and run types

- **Real** runs dispatch activities through execution runtimes such as a Kubernetes or SLURM adapter configured by the environment.
- **Simulation** runs dispatch simulation-capable activities through a simulation runtime such as SimGrid.
- **Interactive** sessions open a terminal against a compatible resource. They are represented in the unified run history, but are opened through the console-session API rather than the planned workflow execution request.

The run history distinguishes `workflow`, `interactive`, and `standalone` kinds.

## Status and timing

A submitted execution first enters the queue. Once the daemon starts it, the workflow run is `running` and then becomes `completed` or `failed`. The domain defines `created`, but the current workflow supervisor does not persist that state during normal execution. Activities expose the more detailed states `blocked`, `ready`, `preparing`, `running`, `completed`, `failed`, and `cancelled`.

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

<img src={require('@site/static/img/interface/runs/workflow-history.png').default} alt="AkôFlow Desktop Runs history filtered to Workflow, showing type, real or simulation mode, status, target, start time and activity progress." />

*Start with the **Type** filter when investigating a workflow. The table then separates real and simulation runs, exposes the target and completion state, and reports activity progress. Select a row or **Details** to move from the compact history into the run evidence.*

<img src={require('@site/static/img/interface/runs/simgrid-run-decomposition.png').default} alt="Completed SimGrid run detail in AkôFlow Desktop with run status, observed makespan, transferred data and the execution-time decomposition chart." />

*In the run detail, **Workflow makespan** is wall-clock completion time. **Accumulated stage time** is the sum of work attributed to stages across activities, so it can be greater than makespan when activities overlap. The decomposition makes transfer, execution, queue, boot and interference visible instead of treating them as a single unexplained duration.*

To open an interactive terminal, use the console action for a compatible resource. The session appears with interactive runs in **Runs** and can be closed or have its log exported.

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
