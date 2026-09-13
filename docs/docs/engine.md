---

id: engine
title: Execution control plane
sidebar_label: Execution control plane
description: How the server queues work, dispatches planning and execution, and records runtime state.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

The AkôFlow server stores requests and dispatches longer work through a durable queue. Planning and execution handlers process those jobs; runtime adapters carry out provider-specific operations.

This is an orchestration explanation, not an API contract. Use the [planning and execution state reference](/docs/reference/planning-and-execution-states) for states and endpoints.

## From request to durable work

<img src={useBaseUrl('/img/architecture/engine-request-dispatch.svg')} alt="An HTTP request is validated and persisted as a typed queue job, then dispatched to planning, execution, cloud, transfer, monitoring or maintenance handlers." />

The API can acknowledge a request before its job starts. In particular, an execution request is accepted into the durable queue; the workflow execution run is created when the daemon begins processing that job. In read-only instance mode the server serves inspection APIs but does not run mutating background work.

## Planning and execution are separate handlers

A planning handler freezes the inputs for a session, invokes registered algorithms, and persists candidates. A user or API client selects one candidate to create the schedule plan used by execution. The [planning explanation](/docs/explanations/planning) covers the significance of that boundary.

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

An `ActivityHandle` carries the provider's external identity, status, endpoints, log, exit result, failure, and artifact observation. The supervisor inspects handles while a run is active. It does not currently reconstruct an interrupted workflow run from persisted handles after a server restart. The [runtime adapters explanation](/docs/runtimes) describes what each current driver does behind this interface.

## Preparation happens before execution

The supervisor resolves executable variants, workspaces, upstream data, and transfer routes before calling a provider. A preparation gate is committed only after required materializations and transfers are verified. A task therefore cannot be considered ready merely because its predecessor process ended: the required input must also be available at the assigned destination.

For real runs, cloud lifecycle actions can be prewarmed before an activity is dispatched. For a simulation run, the frozen request is given to the simulator; real adapters are not started.

## Recovery and failure evidence

Queue jobs retain ownership, attempts, retry timing, and terminal status. Runtime handles, transfers, and materializations are persisted as evidence. The activity controller has a `Stop` method for a handle, but the current API has no workflow-run cancellation endpoint. A failed activity ends the workflow run; the supervisor does not retry that activity. A completed task also records its output observation; a zero exit code is not sufficient if the configured output observation cannot be trusted.

The result is an inspectable distinction between what the plan predicted and what the runtime observed. See [evidence and provenance](/docs/explanations/evidence-and-provenance) for that comparison.
