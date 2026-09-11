---
id: engine
title: Execution control plane
sidebar_label: Execution control plane
description: How the daemon persists work, dispatches planning and execution, and recovers runtime state.
---

AkôFlow's server is a persistent control-plane daemon. The HTTP API validates and stores requests; a durable event loop dispatches work that may take longer than one request. The daemon is therefore responsible for recording intent and state transitions, while runtime adapters perform provider-specific work.

This is an orchestration explanation, not an API contract. Use the [planning and execution state reference](./reference/planning-and-execution-states) for states and endpoints.

## From request to durable work

```text
HTTP request
   |
validate + persist
   |
typed queue job
   |
persistent dispatcher ----> planning handler
                      \---> execution handler
                      \---> cloud, transfer, monitoring, and maintenance handlers
```

The API can acknowledge a request before its job starts. In particular, an execution request is accepted into the durable queue; the workflow execution run is created when the daemon begins processing that job. In read-only instance mode the server serves inspection APIs but does not run mutating background work.

## Planning and execution are separate handlers

A planning handler freezes the inputs for a session, invokes registered algorithms, and persists candidates. A user or API client selects one candidate to create the schedule plan used by execution. The [planning explanation](./explanations/planning) covers the significance of that boundary.

An execution handler validates the selected plan and its bindings, then hands the work to the supervisor. The supervisor follows the workflow DAG: it starts an activity only when its control predecessors have completed and its preparation gate has committed. If incomplete activities remain and nothing can run, the run fails rather than silently assuming a valid schedule.

## Runtime-independent supervision

Every adapter has the same lifecycle boundary:

```go
type RuntimeAdapter interface {
    Modes() []ExecutionMode
    Start(context.Context, ActivityExecutionContext) (ActivityHandle, error)
    Inspect(context.Context, ActivityHandle) (ActivityHandle, error)
    Stop(context.Context, ActivityHandle) error
}
```

An `ActivityHandle` carries the provider's external identity, status, endpoints, log, exit result, failure, and artifact observation. This lets the supervisor recover by inspecting a persisted handle instead of starting an uncertain activity again. The [runtime adapters explanation](./runtimes) describes what each current driver does behind this interface.

## Preparation happens before execution

The supervisor resolves executable variants, workspaces, upstream data, and transfer routes before calling a provider. A preparation gate is committed only after required materializations and transfers are verified. A task therefore cannot be considered ready merely because its predecessor process ended: the required input must also be available at the assigned destination.

For real runs, cloud lifecycle actions can be prewarmed before an activity is dispatched. For a simulation run, the frozen request is given to the simulator; real adapters are not started.

## Recovery and failure evidence

Queue jobs retain ownership, attempts, retry timing, and terminal status. Runtime handles, transfers, and materializations are persisted as evidence. When a provider supports stopping work, cancellation calls its `Stop` method. A task failure ends the run when no permitted retry remains. A completed task also records its output observation; a zero exit code is not sufficient if the configured output observation cannot be trusted.

The result is an inspectable distinction between what the plan predicted and what the runtime observed. See [evidence and provenance](./explanations/evidence-and-provenance) for that comparison.
