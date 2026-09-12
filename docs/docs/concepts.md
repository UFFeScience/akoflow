---
id: concepts
title: Core concepts
sidebar_label: Core concepts
description: Understand workflows, environments, plans, runs, artifacts, and provenance in AkôFlow.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

AkôFlow keeps the workflow you define, the resources available to it, the plan you choose, and the result you observe as separate records. That lets you try a different plan without rewriting the workflow.

## From workflow to result

<img src={useBaseUrl('/img/architecture/record-chain.svg')} alt="A workflow and an environment lead to candidate plans; a selected plan leads to a run, artifacts, and provenance." />

A **workflow** is a set of activities with dependencies. For example, `prepare → analyze → summarize` means that analysis waits for preparation and the summary waits for analysis. The workflow describes the work and required data; it does not choose a machine.

An **environment** describes where work could run: a local host, a modeled simulation platform, a Kubernetes cluster, or an HPC system. Its **resources** are the available machines or capacity. An **execution scope** limits which environment versions and network links a planning experiment can use.

A **plan** assigns activities to resources and predicts timing, transfers, and possibly cost. AkôFlow can produce several candidates, or you can supply a manual plan. Selecting one does not start a run; it records the choice you want to execute.

A **run** records what happened when that plan was submitted. It has activity statuses, observed timing, transfers, and output evidence. Compare the run with its plan to see where prediction and observation differ.

**Executable artifacts** are the versioned programs or images used by activities. **Scientific data** includes inputs and outputs associated with the work. **Provenance** links the workflow, plan, run, activities, and data so you can trace how a result was produced. The audit trail records operational actions separately.

## Where to go next

Start with [the first local workflow in Desktop](./guides/workflows/first-local-run). Then use [Workflow definitions](./guides/workflows/definitions), [Planning](./guides/workflows/planning), and [Execution](./guides/workflows/executions) for the individual tasks. The [SimGrid API tutorial](./guides/workflows/first-run) is a separate simulation example.

For exact file fields, use the [workflow specification](./internal/workflow-spec) and [environment reference](./reference/environment-yaml). For implementation details, see [Architecture internals](./modules).
