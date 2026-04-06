---
id: getting-started
title: Getting Started
sidebar_label: Getting Started
slug: /getting-started
---

# AkôFlow

> *"Akô" means "box" in the Brazilian Tupi language — a container for everything your workflow needs.*

AkôFlow is an open-source engine for modeling and executing **containerized scientific workflows** across heterogeneous infrastructures. It unifies deployment, scheduling, execution, and provenance management within a single runtime — enabling portable, reproducible runs across the **computing continuum**: local machines, on-premise clusters, HPC systems, and multiple cloud providers simultaneously.

It was originally developed within the e-Science Research Group at the Institute of Computing, Fluminense Federal University (UFF), and has been applied in large-scale scientific use cases such as workflows for the *Legacy Survey of Space and Time* (astronomical outlier detection in very large catalogs).

---

## The problem AkôFlow solves

Complex scientific workflows share a set of recurring challenges:

- **Portability** — moving a workflow from a local cluster to AWS, GCP, or an HPC system typically requires significant reconfiguration.
- **Scheduling** — containers sharing the same host compete for CPU and memory. Naïve schedulers either underutilize resources or cause task failures in memory-constrained environments.
- **Provenance** — knowing *exactly* which data transformations produced a given result, on which infrastructure, and with what parameters is essential for reproducibility and debugging — yet most engines treat provenance as an afterthought.
- **Multi-environment deployment** — provisioning infrastructure across cloud providers, HPC clusters, and on-premise nodes manually is error-prone and slow.

AkôFlow addresses all four challenges within one system.

---

## Core ideas

### Workflows as DAGs

In AkôFlow, a workflow is a **Directed Acyclic Graph** (DAG) where:

- **Nodes** are either *data transformations* (tasks) or *data items* (files/artifacts).
- **Edges** encode data dependencies — a transformation cannot start until all its input data is available.

This representation is expressed in a simple YAML specification. Each activity (task) declares the container image it needs, its resource requirements (CPU, memory), and the shell commands to run:

```yaml
name: my-workflow
spec:
  image: "python:3.11"
  namespace: "akoflow"
  activities:
    - name: "preprocess"
      memoryLimit: 1Gi
      cpuLimit: 1.0
      run: |
        python preprocess.py --input /data/raw --output /data/clean

    - name: "train"
      dependsOn: ["preprocess"]
      memoryLimit: 4Gi
      cpuLimit: 2.0
      run: |
        python train.py --data /data/clean --model /data/model
```

### Containerized execution

Every task runs inside a **container** — Docker, Kubernetes Pod, or Singularity image. Containerization gives AkôFlow two key properties:

1. **Portability** — the same container image runs identically on a laptop, an AWS EC2 instance, or an HPC node running Singularity.
2. **Isolation** — tasks are isolated from one another and from the host, making resource accounting predictable.

### Provenance as a first-class citizen

AkôFlow captures **provenance at runtime**, not after the fact. Every file created or consumed, every scheduling decision, every resource utilization measurement is recorded and queryable. The provenance model is compliant with the **W3C PROV** standard, enabling cross-environment lineage inspection and deterministic replay.

---

## What you can do with AkôFlow

| Capability | Description |
|---|---|
| Submit workflows | Define workflows in YAML and submit via CLI, web UI, or REST API |
| Multi-environment execution | Run different tasks in different cloud regions, HPC clusters, or locally — within the same workflow |
| Custom scheduling | Plug in your own `AkôScore` function to balance makespan, memory, cost, or energy |
| Real-time monitoring | Observe task states, resource utilization, and scheduling decisions as execution progresses |
| Provenance queries | Inspect data lineage, trace incorrect outputs to their origin, and replay executions |
| Infrastructure-as-code | Provision entire environments with one command using Terraform-based templates |

---

## Your first workflow in 60 seconds

Start AkôFlow locally, then submit this workflow — it runs three tasks, passes data between them, and completes in under 30 seconds:

```bash
# 1. Start AkôFlow (requires Docker)
akoflow
```

Provision the Local environment kubernetes cluster with kind, then submit the workflow:



```yaml
# Save as hello.yaml
name: wf-hello-world
spec:
  image: "alpine:latest"
  namespace: "akoflow"
  storagePolicy:
    type: distributed
    storageClassName: "hostpath"
    storageSize: "32Mi"
  mountPath: "/data"

  activities:
    - name: "hello-a"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        echo "Hello from task A"
        echo "task-a" > /data/a.txt

    - name: "hello-b"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      run: |
        echo "Hello from task B"
        echo "task-b" > /data/b.txt

    - name: "combine"
      memoryLimit: 128Mi
      cpuLimit: 0.1
      dependsOn: ["hello-a", "hello-b"]
      run: |
        cat /data/a.txt /data/b.txt
        echo "done"
```

`hello-a` and `hello-b` run in parallel. `combine` starts only after both finish.

→ [More examples](examples) — fan-out/fan-in, Python ETL, multi-runtime, HPC, Montage

---

## Documentation sections

### [Modules](modules)
Understand the building blocks of AkôFlow — the Deployment Control Plane, the Workflow Engine, and how they connect.

### [Concepts](concepts)
Deep dive into the workflow model, the AkôScore scheduling formulation, task lifecycle, and the provenance data model.

### [Installation](installation)
Install AkôFlow locally or deploy it across cloud/HPC environments.

### [CLI Reference](cli)
All commands available in the `akoflow` terminal tool.

### [User Guide](user-guide)
Step-by-step guide for submitting and monitoring workflows.

### [Workflow Specification](internal/workflow-spec)
Complete reference for the YAML workflow definition format.
