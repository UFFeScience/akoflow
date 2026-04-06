---
id: engine
title: Workflow Engine Internals
sidebar_label: Engine Internals
---

The Workflow Engine is the component that actually runs your workflows. When it starts, it launches six concurrent **goroutines** (Go's lightweight threads) that cooperate through a shared channel. Understanding how they interact helps you reason about performance, timing, and failure handling.

---

## Startup sequence

When the engine starts, the `main` function launches each goroutine and then blocks on the HTTP server:

```go
// cmd/server/main.go
func main() {
    config.SetupEnv()

    go healthcheck.New().StartHealthCheck()   // ① connects to runtimes
    go worker.New().StartWorker()             // ② executes tasks
    go orchestrator.StartOrchestrator()       // ③ dispatches ready tasks
    go monitor.StartMonitor()                 // ④ collects metrics + state changes
    go garbagecollector.StartGarbageCollector() // ⑤ cleans up storage

    httpserver.StartServer()                  // ⑥ REST API (blocks main)
}
```

All six run concurrently. Here is a map of how they interact:

```
                    ┌──────────────────────────────────────────────────┐
                    │               HTTP Server  ⑥                     │
                    │  (REST API — workflow submission, status, PROV)  │
                    └─────────────────┬────────────────────────────────┘
                                      │ writes to DB
                                      ▼
┌──────────────┐   reads DB    ┌──────────────────┐   writes channel   ┌──────────────┐
│ Orchestrator │──────────────▶│   SQLite DB       │◀──────────────────│   Worker     │
│     ③        │               │  (single source   │   writes DB        │     ②        │
│  every 1s    │               │   of truth)       │                    │  blocks on   │
└──────────────┘               └──────────────────┘                    │  channel     │
                                      ▲                                 └──────┬───────┘
                                      │ writes DB                             │
                               ┌──────┴──────┐                               │ dispatches
                               │   Monitor   │                          ┌─────▼──────────┐
                               │     ④       │                          │  WorfklowChannel│
                               │  every 1s   │                          │  (capacity 1000)│
                               └─────────────┘                          └────────────────┘
                                      ▲
                               ┌──────┴──────┐
                               │HealthCheck  │
                               │     ①       │
                               │  every 5s   │
                               └─────────────┘
                               ┌─────────────┐
                               │  Garbage    │
                               │ Collector ⑤ │
                               │  every 1s   │
                               └─────────────┘
```

---

## Goroutine reference

### ① HealthCheck — every 5 seconds

**Responsibility:** Discover and monitor runtime connectivity.

At startup, the HealthCheck reads all configured runtimes from environment variables and registers them in the database. Then it loops every 5 seconds:

1. For each registered runtime, calls `HealthCheck(runtimeName)` on the appropriate runtime adapter
2. If the runtime responds successfully, marks it `READY` in the database; otherwise marks it `NOT_READY`
3. For Kubernetes runtimes: additionally calls the Metrics API and upserts current CPU/memory usage for each node
4. For Kubernetes runtimes: calls the Nodes API to discover new worker nodes (auto-registration)

```
HealthCheck goroutine
│
├─ [startup] read env vars → register runtimes in DB
│
└─ [every 5s]
    ├─ for each runtime:
    │   ├─ ping /healthz → update status (READY | NOT_READY)
    │   ├─ GET /metrics → update node CPU/memory in DB
    │   └─ GET /nodes → discover and register new nodes
```

Without a healthy runtime, the Orchestrator cannot schedule tasks to it — the HealthCheck is the gatekeeper for node availability.

---

### ② Worker — event-driven

**Responsibility:** Execute individual tasks on the target runtime.

The Worker runs a blocking `for` loop that reads from the shared `WorfklowChannel`. It does not poll — it blocks until a task arrives:

```go
// pkg/server/engine/worker/worker.go
func (w *Worker) StartWorker() {
    for {
        result := <-managerChannel.WorfklowChannel   // blocks here

        runActivityInClusterService.Run(result.Id)   // executes the task
    }
}
```

When a task ID arrives on the channel, the Worker:
1. Loads the activity from the database
2. Selects the appropriate runtime adapter based on the task's `runtime` field
3. Calls `ApplyJob(workflowID, activityID)` on the adapter
4. The adapter translates this into runtime-specific calls (K8s Job, Singularity process, SLURM sbatch, etc.)

The channel has a capacity of **1000 pending tasks** — this acts as a backpressure buffer between the Orchestrator and the Worker.

---

### ③ Orchestrator — every 1 second

**Responsibility:** Evaluate DAG dependencies and dispatch Ready tasks.

The Orchestrator is the brain of execution. Every second it:

1. Fetches all workflows in `Pending` or `Running` state from the database
2. For each workflow, computes which tasks are **Ready** (all dependencies `Finished`)
3. Runs those Ready tasks through the **AkôScore scheduler** to assign each to a node
4. Records the scheduling decision in the database
5. Sends the task ID to the `WorfklowChannel` for the Worker to pick up

```
Orchestrator loop (every 1s)
│
└─ for each pending/running workflow:
    ├─ load tasks and their statuses
    ├─ compute ready tasks (dependsOn all Finished?)
    ├─ for each ready task:
    │   ├─ AkôScore: evaluate (task, node) pairs
    │   ├─ select best node
    │   ├─ write schedule decision to DB
    │   └─ send task ID → WorfklowChannel
```

The Orchestrator never blocks waiting for tasks to complete — it fires and forgets onto the channel, then checks again in the next cycle.

---

### ④ Monitor — every 1 second

**Responsibility:** Track state changes and collect runtime metrics.

The Monitor runs two sub-services every second:

**MonitorChangeWorkflow** — checks if running tasks have completed:
- For each running task, asks the runtime adapter if the task has finished (`VerifyActivitiesWasFinished`)
- If finished, updates the task status to `Finished` in the database
- This status change will be picked up by the Orchestrator in its next cycle to unlock downstream tasks

**MonitorCollectMetrics** — collects resource utilization:
- For each running task, calls `GetMetrics` and `GetLogs` on the runtime adapter
- Inserts a `Metrics` record in the database (CPU %, memory %, wall time, timestamp)
- Inserts `Logs` records for stdout/stderr
- This data feeds the provenance store and the live monitoring UI

```
Monitor loop (every 1s)
│
├─ MonitorChangeWorkflow:
│   └─ for each running task → check finished? → update DB status
│
└─ MonitorCollectMetrics:
    └─ for each running task → collect CPU/mem/logs → insert Metrics + Logs in DB
```

---

### ⑤ GarbageCollector — every 1 second

**Responsibility:** Clean up storage volumes after workflow completion.

When a workflow finishes, its associated storage volumes (PVCs, local directories) need to be removed. The GarbageCollector finds completed workflows with lingering storage and deprovisions them.

---

### ⑥ HTTP Server — main goroutine

**Responsibility:** Expose the REST API for workflow management.

The HTTP Server is the entry point for users and the Control Plane. It handles:
- Workflow submission (POST with base64-encoded YAML)
- Workflow status queries
- Activity and metrics queries
- Provenance graph queries
- Runtime and schedule management

---

## The WorfklowChannel

The channel is the communication backbone between the Orchestrator and the Worker:

```go
// pkg/server/engine/channel/channel.go

type Manager struct {
    WorfklowChannel chan DataChannel   // buffered, capacity 1000
}

type DataChannel struct {
    Namespace string
    Job       interface{}
    Id        int      // activity ID — the Worker loads the rest from DB
}
```

Key properties:
- **Buffered** (capacity 1000) — the Orchestrator can enqueue up to 1000 tasks without the Worker having processed them yet
- **Singleton** — there is exactly one channel instance per Engine, protected by a mutex
- **ID-only** — only the activity ID is sent through the channel; the Worker loads the full task specification from the database, avoiding serialization overhead

If the channel is full, the Orchestrator's `dispatchToWorker` call will block until the Worker drains a slot. This is the natural back-pressure mechanism.

---

## Timing model

| Goroutine | Interval | Purpose |
|---|---|---|
| HealthCheck | 5 seconds | Runtime connectivity + node discovery |
| Worker | Event-driven | Executes tasks as they arrive |
| Orchestrator | 1 second | DAG evaluation + task dispatch |
| Monitor | 1 second | State sync + metrics collection |
| GarbageCollector | 1 second | Storage cleanup |
| HTTP Server | Always on | REST API |

The minimum latency from a task becoming Ready to its first container action is approximately **1–2 seconds** (one Orchestrator cycle to detect it, one Worker cycle to execute it).

---

## SQLite as the shared state store

All goroutines communicate indirectly through the **SQLite database** (not through shared memory or direct Go channels). This design choice means:

- Any goroutine can read/write state independently
- State survives engine restarts (crash recovery)
- The provenance data is co-located with execution state

The tradeoff is that SQLite serializes writes, which is acceptable for the current workload but would become a bottleneck at very high task throughput.
