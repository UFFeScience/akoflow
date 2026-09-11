---
id: engine
title: Control plane and execution
sidebar_label: Control plane and execution
---

The current AkôFlow server is a control-plane daemon. Older descriptions of one engine per environment, a global in-memory `WorfklowChannel`, fixed orchestrator goroutines, or SQLite aggregation across engines no longer describe the implementation.

## Process lifecycle

At startup the daemon opens operational and analytics persistence, builds credential stores, provider registries, planning algorithms, transfer connectors, cloud services, API handlers, and the persistent event loop. In writable mode it also starts connection monitoring. The API is the synchronous entry point; long-running work is queued.

```text
HTTP request
   +-- validate and persist
   +-- enqueue typed job
   v
persistent queue -> dispatcher -> planning | execution | cloud handler
                                      |
                               repositories + providers
```

Read-only instance mode serves inspection APIs without starting mutating background processing.

## Planning flow

1. Create a planning session for a workflow version and execution scope.
2. The coordinator freezes versions, resources, topology, profiles, constraints, interference data, and algorithms.
3. A planning job is dispatched.
4. Registered algorithms generate candidates and predicted metrics.
5. Compare candidates and select one to obtain the plan used by execution.

Built-ins are HEFT, PRISM Time, and PRISM Cost. External plugins are validated. Clients should read algorithm availability from the API.

## Execution flow

Starting a run persists its request and queues an execution job. The supervisor then:

1. validates the plan, assignments, runtime bindings, and DAG;
2. prewarms planned cloud targets for real execution;
3. indexes activities, resources, assignments, and dependencies;
4. inspects running handles and identifies dependency-ready work;
5. prepares executable and workspace data;
6. dispatches work up to the parallel limit;
7. persists task, log, artifact, timing, and transfer observations;
8. completes or fails the run and releases ephemeral cloud allocations.

The current defaults are eight parallel activities and one-second inspection, but these are server defaults rather than API guarantees.

## Runtime-independent control

```go
type RuntimeAdapter interface {
    Modes() []ExecutionMode
    Start(context.Context, ActivityExecutionContext) (ActivityHandle, error)
    Inspect(context.Context, ActivityHandle) (ActivityHandle, error)
    Stop(context.Context, ActivityHandle) error
}
```

The resolver selects an adapter using execution mode and runtime identity. An `ActivityHandle` hides provider identifiers such as a PID, Kubernetes Job, Docker container, Slurm job, or simulation event while carrying status, endpoints, log, exit code, failure, and artifacts.

## DAG progress

Readiness is calculated from dependencies and completed tasks; there is no public `Ready` task state. If nothing is running, nothing is ready, and incomplete activities remain, the run fails as a cycle or incomplete plan.

Interactive runs return a running trace after a supported activity starts so its session can be used. Simulation sends the frozen request to the simulator and never invokes real adapters.

## Preparation gate and transfers

Providers must not start until `PreparationGate` is ready. Requirements can include a resolved executable variant, workspace destination, upstream data, and verified transfers. The coordinator resolves endpoints, selects a route/connector, supports chunk and resume metadata, and checks digests. `committed` means verified bytes match the expected digest, not merely that a file exists.

## Artifact observation

Where supported, adapters snapshot the workspace before and after an activity. The manifest of created or changed files is persisted into the data/provenance model. Observation errors remain execution evidence and can fail a zero-exit activity when outputs cannot be trusted.

## Cloud orchestration

Cloud lifecycle uses the persistent queue. Plans can contain lifecycle actions and provisioned targets. The supervisor prewarms each target, waits for operations, binds the instance/runtime allocation, and releases it after a real run. Cleanup failure is an infrastructure result and does not invalidate completed computation.

## Failure and recovery

- Queue jobs persist ownership, attempts, retry timing, and terminal status.
- Runtime handles are saved so recovery inspects instead of blindly duplicating work.
- Timeout and retry policy belongs to the activity definition.
- Transfer/materialization failures remain first-class records.
- Cancellation calls adapter stop where supported.
- A failed activity fails its run when no valid retry remains.

Desktop and API expose the same stored evidence: predicted-versus-observed timing, assignments, logs, exits, artifacts, transfer routes/bytes/cost, cloud operations, provenance, and audit.
