---
id: getting-started
title: Getting started
sidebar_label: Getting started
slug: /getting-started
description: Understand the current AkôFlow workflow from infrastructure to reproducible execution.
---

# AkôFlow

AkôFlow models, plans, and executes containerized scientific workflows across local, cloud, Kubernetes, HPC, and simulated infrastructure. The same control-plane records connect infrastructure, scheduling decisions, observed execution, artifacts, and scientific provenance.

## The AkôFlow lifecycle

AkôFlow intentionally separates definition, planning, and execution:

```text
Environment and resources
          ↓
Execution scope and network topology
          ↓
Versioned workflow definition
          ↓
Planning session and candidate comparison
          ↓
Selected schedule plan
          ↓
Real, simulated, or interactive execution
          ↓
Observed metrics, artifacts, audit, and provenance
```

This separation makes an experiment reproducible. A plan identifies the workflow version and infrastructure boundary it was built for; a run preserves the selected plan and what actually happened.

## Core records

| Record | Purpose |
|---|---|
| Environment | Owns connected or simulated infrastructure and its versioned inventory |
| Resource | Represents schedulable compute or storage discovered in an environment |
| Execution scope | Selects the environments and network boundary available to planning |
| Workflow definition | Owns a versioned activity DAG and its executable or simulation requirements |
| Planning session | Runs one or more algorithms against a frozen planning input |
| Plan candidate | Preserves one algorithm result for comparison |
| Schedule plan | Canonical assignment selected for execution |
| Execution run | Records a real, simulated, interactive, or standalone run |
| Artifact and data record | Tracks executable materialization and scientific outputs |

## Desktop and API

The Desktop application and HTTP API operate on the same records.

- Use **AkôFlow Desktop** to connect infrastructure, inspect graphs, compare candidates, watch progress, and investigate results.
- Use the **API** to automate experiments, import definitions, create runs, and query results reproducibly.

Task guides show both paths whenever the feature is available in both surfaces.

## Start here

1. [Install AkôFlow](./installation).
2. [Tour the Desktop interface](./guides/interface-tour).
3. [Complete your first end-to-end run](./guides/workflows/first-run).
4. Learn how to manage [environments](./guides/infrastructure/environments) and [workflow definitions](./guides/workflows/definitions).
5. Use the [API overview](./reference/api-overview) for automation.

## What AkôFlow can show

- Connected and simulated infrastructure inventories.
- Activity DAGs and data dependencies.
- Multiple planning algorithms and candidate plans.
- Predicted makespan and cost before execution.
- Planned and observed Gantt timelines.
- Compute, queue, transfer, interference, overhead, storage, and network effects.
- Cloud capacity and infrastructure lifecycle operations.
- Executable artifact builds and materializations.
- Scientific lineage and an operational audit trail.

The guides use screenshots for spatial context and short walkthroughs for state transitions. Every visual procedure also has complete text instructions.
