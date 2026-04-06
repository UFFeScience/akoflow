---
id: concepts
title: Concepts
sidebar_label: Concepts
---

This page explains the key concepts behind AkôFlow: how workflows are modeled, how tasks are scheduled, how execution state is managed, and how provenance is captured. Understanding these concepts helps you write better workflows and make informed decisions about scheduling and deployment.

---

## The workflow model

### Workflows as DAGs

AkôFlow represents workflows as **Directed Acyclic Graphs (DAGs)**. Formally, a workflow is a graph `G = (V, A, a, ω)` where:

- `V = N ∪ D` — the node set, composed of **transformations** `N` (tasks) and **data items** `D` (files/artifacts)
- `A` — directed edges encoding data dependencies between transformations and data items
- `a_i` — computational cost associated with each transformation `i ∈ N`
- `ω_ij` — communication cost associated with edge `(i, j) ∈ A`

In practice, this means:
- Every task in the graph is always preceded and succeeded by a data item, reflecting a structured dataflow
- A task cannot start until all its input data items are available (i.e., all upstream tasks have finished producing them)
- Tasks with no dependencies between them can run in parallel

### YAML representation

You express this DAG in a YAML file. The `activities` list defines the transformation nodes, and the optional `dependsOn` field encodes the edges:

```yaml
name: etl-pipeline
spec:
  image: "python:3.11"
  namespace: "akoflow"
  mountPath: "/data"
  activities:
    - name: "ingest"
      cpuLimit: 0.5
      memoryLimit: 512Mi
      run: |
        python ingest.py --source /data/raw --output /data/ingested

    - name: "deduplicate"
      dependsOn: ["ingest"]
      cpuLimit: 1.0
      memoryLimit: 1Gi
      run: |
        python dedup.py --input /data/ingested --output /data/clean

    - name: "predict-us"
      dependsOn: ["deduplicate"]
      cpuLimit: 2.0
      memoryLimit: 4Gi
      run: |
        python predict.py --region us --input /data/clean --output /data/results-us

    - name: "predict-eu"
      dependsOn: ["deduplicate"]
      cpuLimit: 2.0
      memoryLimit: 4Gi
      run: |
        python predict.py --region eu --input /data/clean --output /data/results-eu

    - name: "aggregate"
      dependsOn: ["predict-us", "predict-eu"]
      cpuLimit: 0.5
      memoryLimit: 512Mi
      run: |
        python aggregate.py --inputs /data/results-us /data/results-eu --output /data/final
```

In this example:
- `ingest` has no dependencies — it runs immediately
- `deduplicate` runs after `ingest` finishes
- `predict-us` and `predict-eu` both depend on `deduplicate` but not on each other — they run in **parallel**
- `aggregate` waits for both predictions to complete

---

## Task lifecycle

Every task in AkôFlow moves through a well-defined sequence of states:

```
  Pending
     │
     │  (all dependencies finished)
     ▼
  Ready ──→ (enters scheduling queue)
     │
     │  (AkôScore selects a node)
     ▼
  In Execution ──→ (container running on a Worker)
     │
     ├──→  Finished  (outputs available for downstream tasks)
     │
     └──→  Failed    (fault-handling may apply)
```

| State | Meaning |
|---|---|
| **Pending** | Task is waiting for one or more upstream tasks to finish |
| **Ready** | All dependencies are satisfied; task is in the scheduling queue |
| **In Execution** | Task has been assigned to a node and its container is running |
| **Finished** | Task completed successfully; its outputs are available to downstream tasks |
| **Failed** | Task encountered an error during execution |

The Orchestrator runs on a continuous loop, checking for newly Ready tasks and dispatching them to Workers via the AkôScore scheduler.

---

## Execution strategies

AkôFlow supports two execution strategies that control when tasks become eligible for scheduling:

### First-Data-First (FDF)

Tasks are dispatched **as soon as their input data becomes available**. This enables pipelined execution — downstream tasks can start processing partial results while upstream tasks are still running on other data partitions. FDF maximizes resource utilization and is well-suited for streaming or data-parallel workflows.

### First-Activity-First (FAF)

AkôFlow enforces **synchronization at each level of the workflow**. All tasks at a given DAG depth must complete before any task at the next depth starts. FAF provides simpler execution semantics and is useful when strict data consistency between levels is required.

---

## AkôScore — the scheduling function

### Why scheduling matters for containerized workflows

Containers executing on the same host share its CPU and memory. Poor scheduling leads to two failure modes:
- **Underutilization** — resources sit idle, making executions slower and more expensive (especially in pay-as-you-go cloud environments)
- **Overutilization** — too many memory-hungry tasks land on the same node, causing performance degradation or task failures

Scheduling is especially hard because the memory consumption of a task can vary substantially depending on the characteristics of its input data — making static, pre-computed schedules unreliable.

### The AkôScore formulation

AkôScore is evaluated **at scheduling time**, for each (task, node) pair. For a task `i` ready to run, AkôFlow computes a score for every available node `j`:

```
S(i,j) = A(i,j) × [ α × (M_free(j) − M_req(i)) / M_max
                   + (1−α) × 1 / T(i,j) ]
```

Where:
- `A(i,j)` — feasibility indicator: `1` if node `j` has enough free CPU and memory for task `i`, `0` otherwise
- `α` — user-defined weight (`0` to `1`): controls the trade-off between memory and speed
- `M_free(j)` — memory currently available on node `j`
- `M_req(i)` — memory required by task `i`
- `M_max` — maximum memory capacity among all nodes (used for normalization)
- `T(i,j)` — estimated execution time of task `i` on node `j`, based on the node's processing capacity

The node with the **highest AkôScore** is selected. If all nodes score `0` (none has sufficient resources), the task stays in the Ready queue until the next scheduling cycle.

### Tuning α

| α value | Effect |
|---|---|
| `0.0` | Minimize makespan — assign tasks to the fastest available node |
| `1.0` | Maximize memory fit — prefer nodes where memory availability closely matches task requirements |
| `0.5` | Balanced — equal weight to speed and memory |

### Per-environment scheduling policies

A key feature of AkôScore is that it can be configured **per environment**. This means the same workflow can use different scheduling strategies across its deployment targets:

- In a cloud environment: optimize for monetary cost
- In an HPC cluster: optimize for energy consumption
- Locally: optimize for makespan (to finish quickly during development)

### Custom scheduling via plugins

AkôScore is extensible. The Engine loads scheduling functions at runtime as **Go plugins** (compiled `.so` files). To implement a custom policy:

1. Write a Go function with the signature `func AkoScore(input any) float64`
2. Compile it as a shared library
3. Register it in AkôFlow and reference it by name in your workflow YAML

```yaml
spec:
  schedule: "my-cost-aware-policy"
```

The Engine will call your function with a map containing the task's resource requirements and the node's current state, and use the returned score for placement decisions.

---

## Provenance

### Why provenance matters

Provenance answers questions like:
- *Which transformation produced this output file?*
- *What input data was used, and where did it come from?*
- *Which node ran this task, at what time, and with what resource utilization?*
- *If an output is wrong, which tasks and data artifacts are implicated?*

AkôFlow treats provenance as a **first-class component** — not an optional logging feature. It is captured continuously at runtime, across all environments, and queryable at any point during or after execution.

### The provenance data model

AkôFlow's provenance is stored in a five-table relational schema (SQLite per Engine instance, aggregated by the Control Plane):

| Table | What it records |
|---|---|
| `Workflow` | Submission metadata: namespace, spec file path, execution state, timestamps |
| `Activity` | Each task execution: parent workflow, assigned node, state, scheduling metadata |
| `Metrics` | Resource utilization samples collected every 15 seconds per running task: CPU %, memory, wall-clock time |
| `Errors` | Standard output and error streams for each task, preserving the full execution trace |
| `Files` | All data artifacts produced during execution |

### Data lineage graph

From the `Files` and `Activity` tables, AkôFlow constructs a **data lineage graph** — a W3C PROV-compliant directed graph:

```
[input-file] ──used──▶ [task-A] ──wasGeneratedBy──▶ [output-file-A]
                                                            │
                                                          used
                                                            ▼
                                                        [task-B] ──wasGeneratedBy──▶ [final-output]
```

The graph is built by diffing the file system state **before and after** each task runs:
- Files present at the end but not at the start → **produced** by the task (`wasGeneratedBy`)
- Files present at the start → **consumed** by the task (`used`)

This approach requires no instrumentation of user code — provenance is captured transparently at the storage layer.

### Three levels of provenance

AkôFlow captures provenance at three levels:

| Level | What it tracks |
|---|---|
| **Data-level** | Relationships between data artifacts and transformations (lineage graph) |
| **Execution-level** | Runtime behavior: task states, container placement, scheduling decisions, resource utilization |
| **Environment-level** | Infrastructure context: node configuration, runtime type, scheduling parameters used |

### Querying provenance

Provenance data can be explored through:
- **The web UI** — visual lineage graphs with filtering by task or artifact
- **SQL queries** — direct queries against the SQLite provenance database for targeted inspection
- **W3C PROV export** — export the full graph in PROV-compliant format for use with external tools

---

## The computing continuum

AkôFlow is designed for execution across the **computing continuum** — the full spectrum of computational infrastructure from edge devices to supercomputers:

```
Local machine → On-premise cluster → HPC system → Private cloud → Public cloud (AWS, GCP, ...)
```

Most workflow engines are optimized for one point on this spectrum. AkôFlow abstracts infrastructure-specific dependencies through containerization and Terraform-based provisioning, allowing the same workflow to run across any combination of these environments.

A practical example: a workflow processing user data under GDPR must keep European data in EU-based infrastructure. AkôFlow can distribute the same DAG across environments — EU-region cloud for European data, US-region cloud for American data — with the Control Plane coordinating execution and aggregating results.

### Supported environments

| Environment | Runtime |
|---|---|
| Local machine | Docker |
| On-premise cluster | Docker, Kubernetes |
| HPC system | Singularity (where Docker is unavailable) |
| AWS | Kubernetes (EKS), Docker |
| GCP | Kubernetes (GKE), Docker |
| Any Kubernetes cluster | Kubernetes |
