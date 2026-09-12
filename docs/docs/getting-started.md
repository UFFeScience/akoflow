---
id: getting-started
title: Getting started with AkôFlow
sidebar_label: Getting started
slug: /getting-started
description: Understand AkôFlow, choose a supported path, and find the next workflow task.
---

# Getting started with AkôFlow

AkôFlow helps you define a scientific workflow, choose where its activities run, compare plans, and inspect the results. A workflow describes activities and their data dependencies. A run records what happened after a plan was selected.

Start with a local environment to learn the interface. Connecting HPC, Kubernetes, or cloud resources requires the access and checks in their own guides.

## Start with Desktop

1. [Install AkôFlow Desktop](./installation) and complete its local environment checkup.
2. [Run your first local workflow](./guides/workflows/first-local-run) and inspect its result in Desktop.
3. [Tour the interface](./guides/interface-tour) or continue with [Workflow definitions](./guides/workflows/definitions), [Planning](./guides/workflows/planning), and [Execution](./guides/workflows/executions).

The [checked-in SimGrid example](./guides/workflows/first-run) verifies three activities and two transfers through a separately managed API endpoint. Use it after the local Desktop run if you want to explore simulation.

## Connect another environment

- [Register HPC / SLURM](./tutorials/register-hpc) after receiving site-approved SSH and scheduler access.
- [Connect Google Cloud](./tutorials/connect-cloud) with a service account and a project you can inspect.
- [Configure Kubernetes](./guides/infrastructure/kubernetes) when you have cluster access.
- [Review cloud support](./guides/infrastructure/cloud-capacity#provider-support-in-v10) before planning a cloud run. AWS EC2 discovery and provisioning are not implemented.

For direct API work, complete [API connection setup](./tutorials/api-access) first.

## I already have AkôFlow running

| Goal | Continue with |
| --- | --- |
| Define or import an activity DAG | [Workflow definitions](./guides/workflows/definitions) |
| Generate PRISM or HEFT candidates, or place activities manually | [Plan a workflow](./guides/workflows/planning) |
| Start a selected plan and inspect observed evidence | [Execute and monitor a workflow](./guides/workflows/executions) |
| Configure simulated or connected infrastructure | [Environments](./guides/infrastructure/environments) |
| Limit the resources and network offered to planning | [Execution scopes and network topologies](./guides/infrastructure/execution-scopes) |
| Reproduce a complete example | [Workflow Showcase](./showcase/) |
| Query lineage, evidence, or audit records | [Provenance and audit](./guides/data/provenance-and-audit) |

## Understand the records

Read [Core concepts](./concepts) for workflow, environment, plan, run, artifacts, and provenance. For implementation details, see [Architecture internals](./modules). The [API overview](./reference/api-overview) and [workflow specification](./internal/workflow-spec) are reference material for automation.

## When something fails

Use [Troubleshooting](./guides/operations/troubleshooting) for daemon readiness, authentication, Docker and BuildKit checks, connection failures, and diagnostic collection. For remote targets, validate credentials and the environment connection before debugging the workflow itself.
