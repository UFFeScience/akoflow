---
id: getting-started
title: Getting started with AkôFlow
sidebar_label: Getting started
slug: /getting-started
description: Understand AkôFlow, choose a supported path, and find the next workflow task.
---

# Getting started with AkôFlow

AkôFlow helps you define a scientific workflow, choose where its activities run, make a plan, and execute it. A workflow describes activities and data dependencies. A run records what happened when you executed the plan. You can compare planning choices after the first run.

Start with a local environment to learn the interface. Connecting HPC, Kubernetes, or cloud resources requires the access and checks in their own guides.

## Start with Desktop

1. [Install AkôFlow Desktop](/docs/installation) and complete its local environment checkup.
2. [Run your first local workflow](/docs/guides/workflows/first-local-run) and inspect its result in Desktop.
3. [Tour the interface](/docs/guides/interface-tour) or continue with [Workflow definitions](/docs/guides/workflows/definitions), [Planning](/docs/guides/workflows/planning), and [Execution](/docs/guides/workflows/executions).

The [checked-in SimGrid example](/docs/guides/workflows/first-run) verifies three activities and two transfers through a separately managed API endpoint. Use it after the local Desktop run if you want to explore simulation.

## What is supported today

The local Desktop workflow has a verified run on Linux. The [Showcase](/docs/showcase) also has verified SimGrid and Kubernetes-on-Kind examples. The SLURM example exercises a local scheduler fixture; a run on an institutional cluster remains unverified.

Google Cloud catalog and worker provisioning are implemented, but a complete live provision-and-destroy cycle has not been verified. AWS EC2 discovery and provisioning are unavailable; S3 transfers have local code tests but no verified AWS-account run. Check [cloud provider support](/docs/guides/infrastructure/cloud-support) before choosing a provider.

## Connect another environment

- [Register HPC / SLURM](/docs/tutorials/register-hpc) after receiving site-approved SSH and scheduler access.
- [Connect Google Cloud](/docs/tutorials/connect-cloud) with a service account and a project you can inspect.
- [Configure Kubernetes](/docs/guides/infrastructure/kubernetes) when you have cluster access.

For direct API work, complete [API connection setup](/docs/tutorials/api-access) first.

## I already have AkôFlow running

| Goal | Continue with |
| --- | --- |
| Define or import an activity DAG | [Workflow definitions](/docs/guides/workflows/definitions) |
| Generate PRISM or HEFT candidates, or place activities manually | [Plan a workflow](/docs/guides/workflows/planning) |
| Start a selected plan and inspect observed evidence | [Execute and monitor a workflow](/docs/guides/workflows/executions) |
| Configure simulated or connected infrastructure | [Environments](/docs/guides/infrastructure/environments) |
| Limit the resources and network offered to planning | [Execution scopes and network topologies](/docs/guides/infrastructure/execution-scopes) |
| Reproduce a complete example | [Workflow Showcase](/docs/showcase) |
| Trace how a result was produced | [Trace a result with provenance](/docs/guides/data/provenance) |
| Investigate a connection check, resource discovery, or console action | [Inspect audit events](/docs/guides/data/audit-events) |

## Understand the records

Read [Core concepts](/docs/concepts) for workflow, environment, plan, run, artifacts, and provenance. For implementation details, see [Architecture internals](/docs/modules). The [API overview](/docs/reference/api-overview) and [workflow specification](/docs/internal/workflow-spec) are reference material for automation.

## When something fails

Use [Troubleshooting](/docs/guides/operations/troubleshooting) for server readiness, authentication, Docker and BuildKit checks, connection failures, and diagnostic collection. For remote targets, validate credentials and the environment connection before debugging the workflow itself.
