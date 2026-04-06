---
id: modules
title: Modules
sidebar_label: Modules
---

AkôFlow is structured around two main blocks — a **Deployment Control Plane** and a **Workflow Engine** — plus client interfaces (web UI, desktop app, and CLI). Each block has a clear responsibility, and they compose to cover the full lifecycle of a workflow across heterogeneous environments.

---

## Architecture overview

```
                    ┌─────────────────────────────────────────┐
                    │        Your machine / browser            │
                    │                                          │
                    │   akoflow CLI  ──or──  Desktop App       │
                    │          │                   │           │
                    │          └────────┬──────────┘           │
                    └───────────────────┼─────────────────────┘
                                        │
                                        ▼
                    ┌─────────────────────────────────────────┐
                    │        Deployment Control Plane          │
                    │                                          │
                    │  • Declarative environment definitions   │
                    │  • Infrastructure provisioning (Terraform)│
                    │  • Provenance aggregation & monitoring   │
                    └─────────────────────────────────────────┘
                                        │
                      ┌─────────────────┼─────────────────┐
                      │                 │                  │
                      ▼                 ▼                  ▼
             ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
             │  Workflow     │  │  Workflow     │  │  Workflow     │
             │  Engine       │  │  Engine       │  │  Engine       │
             │  (local)      │  │  (AWS / GCP)  │  │  (HPC/on-prem)│
             └──────────────┘  └──────────────┘  └──────────────┘
                    │                 │                  │
              containers         containers          containers
              (Docker)           (Kubernetes)        (Singularity)
```

The **Control Plane** provisions environments and deploys a **Workflow Engine** inside each one. Tasks then run as containers **where the data and resources are** — not necessarily on your machine. A single workflow can span all three environments simultaneously.

---

## Deployment Control Plane

The Control Plane is the central hub for environment management. It is responsible for:

### Environment specification and provisioning

Users describe their target infrastructure using **Terraform HCL templates**. AkôFlow ships an *Environment Template Catalog* — pre-built templates for common configurations (AWS EC2, GCP Compute Engine, HPC clusters, local Docker) — so you don't have to write Terraform from scratch.

An environment specification declares:
- The set of compute nodes and their properties (CPU, memory, storage)
- The runtime to use (Docker, Kubernetes, Singularity)
- Cloud credentials and access grants

The Control Plane validates the specification and then calls the appropriate cloud APIs or HPC provisioners to instantiate the resources.

### Workflow Engine deployment

Once an environment is ready, the Control Plane automatically deploys a **Workflow Engine** instance inside it. The Engine runs as a distributed application — multiple instances can operate concurrently across environments.

### Lifecycle management

The Control Plane continuously monitors system state and execution progress across all environments. It handles failures and deprovisions resources when a workflow completes, ensuring you don't pay for idle infrastructure.

### Provenance aggregation

Each Workflow Engine captures provenance locally. The Control Plane's **Provenance Ingestion Engine** aggregates these distributed records into a centralized **Provenance Metadata Repository**, giving you a unified view of data lineage across all environments.

---

## Workflow Engine

Each deployed environment hosts a Workflow Engine instance. This is the component that actually **runs your workflows**.

### Submission and task management

The Engine exposes a **submission gateway** where users (or the Control Plane) can:
- Submit workflow definitions (YAML-based DAGs)
- Monitor execution status in real time
- Configure scheduling policies for the environment

### DAG-driven orchestration

The Engine parses the YAML workflow into an internal DAG representation. An **Orchestrator** loop runs continuously, checking which tasks are ready (i.e., all their dependencies have finished) and dispatching them for execution.

A task moves through these states:

```
Pending → Ready → In Execution → Finished
                              ↘ Failed
```

- **Pending** — waiting for upstream dependencies to complete
- **Ready** — all dependencies satisfied; eligible for scheduling
- **In Execution** — assigned to a node, container running
- **Finished** — completed successfully; outputs available for downstream tasks
- **Failed** — execution error; fault-handling strategies may apply

### AkôScore — customizable scheduling

When a task becomes Ready, the Engine's **Distributed Workflow Scheduler** must assign it to a compute node. This decision is made by evaluating the **AkôScore** for each available node.

AkôScore is a pluggable, multi-objective cost function. The default formulation balances two objectives:

- **Makespan** — minimize execution time by preferring faster nodes
- **Memory utilization** — prefer nodes where available memory closely matches what the task requires, avoiding both under- and over-utilization

Users control the trade-off through a weight parameter `α ∈ [0,1]`:
- `α = 0` → optimize purely for speed (makespan)
- `α = 1` → optimize purely for memory fit
- `α = 0.5` → balanced

A task is only scheduled on a node if both CPU and memory constraints are satisfied. If no node has sufficient resources, the task stays in the Ready queue and is reconsidered in the next scheduling cycle.

Critically, **AkôScore is extensible**: users can provide a custom scheduling function compiled as a Go plugin (`.so` file). This means you can implement domain-specific objectives — minimizing monetary cost in cloud environments, minimizing energy in HPC systems, or any other policy — without modifying the engine's source code.

```yaml
# Example: reference a custom schedule in your workflow spec
spec:
  schedule: "my-cost-aware-policy"
```

### Container execution

The Engine dispatches ready tasks to **Worker** nodes through **Execution Gateway Nodes**. Each Worker:
- Instantiates a container (Docker, Kubernetes Pod, or Singularity)
- Attaches a persistent data volume for intermediate and final files
- Executes the task's commands inside the container
- Reports status back to the Engine

AkôFlow currently supports the following container runtimes:

| Runtime | Typical environment |
|---|---|
| Docker | Local machines, on-premise servers |
| Kubernetes | Cloud clusters, managed K8s (EKS, GKE) |
| Singularity | HPC systems (where Docker is unavailable) |
| Local | Lightweight local execution without containerization |

### Provenance capture

Each Worker runs a **Local Provenance Logger** that records:
- Which files existed before and after each task ran (tracking what was *consumed* vs. *produced*)
- Execution state transitions (Pending → Ready → Running → Finished)
- Resource utilization samples (CPU, memory) collected every 15 seconds

The provenance data is stored in a **five-table relational schema** (SQLite) and exposed in **W3C PROV-compliant format**:

| Table | What it records |
|---|---|
| `Workflow` | Submission metadata, namespace, spec path, state |
| `Activity` | Each task execution, linked to its workflow and node |
| `Metrics` | Per-task resource utilization samples (CPU, memory, wall time) |
| `Errors` | Standard output and error streams from each task |
| `Files` | All data artifacts produced during execution |

From these tables, AkôFlow can reconstruct a **data lineage graph**: nodes are tasks and files, edges are `wasGeneratedBy` (task → file) and `used` (file → task) relationships.

### Execution telemetry

The **Execution Telemetry & State Monitor** observes workflow execution in real time, capturing events such as data production and resource utilization. This information feeds back into scheduling decisions and failure handling.

---

## Desktop App

A native application for **macOS, Windows, and Linux** that provides the same interface as the web-based Control Plane — without needing to open a browser. Useful when you want notifications, faster access, or are working offline.

The Desktop App connects to a running Control Plane instance (local or remote). It does **not** contain the engine or the server.

Download: [latest release →](https://github.com/UFFeScience/akoflow/releases/latest)

---

## akoflow CLI

The `akoflow` command is the fastest way to get started. It launches a full AkôFlow environment on your local machine with one command:

```bash
akoflow     # pulls and starts everything locally
```

Under the hood, it runs the `akoflow/akoflow` Docker image, which bundles the Control Plane and a local Workflow Engine together — giving you the full experience without configuring any cloud resources first.

→ See [CLI Reference](cli) for all commands.

---

## Execution across environments

A single workflow can run across **multiple environments at once**. For example:

- Tasks that process data under European privacy regulations run in a European cloud region
- Tasks that require GPU acceleration run on an HPC cluster
- Lightweight pre-processing runs locally

The Control Plane coordinates placement; each Engine executes independently and reports back. This enables hybrid execution patterns driven by data locality, regulatory constraints, cost, or performance requirements.

→ See [Concepts](concepts) for a deeper look at the scheduling model and provenance.

---