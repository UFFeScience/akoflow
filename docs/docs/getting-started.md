---
id: getting-started
title: Getting started with AkôFlow
sidebar_label: Getting started
slug: /getting-started
description: Understand AkôFlow, its current paths, and how to run a first workflow.
---

# Getting started with AkôFlow

AkôFlow helps you define a scientific workflow, decide where its activities run, and inspect what happened. A workflow describes activities and their data dependencies. AkôFlow can compare plans, run a selected plan, and keep its results and provenance together.

The simplest place to begin is a local simulation. It lets you follow a workflow from definition to results without access to a cluster or cloud account.

## What can I do today?

- **Simulate a workflow locally** with SimGrid and inspect activity and transfer results.
- **Run on connected infrastructure** using the documented Kubernetes or HPC/SLURM paths, after configuring that environment.
- **Explore cloud integration** with the limits described in the [cloud support matrix](./guides/infrastructure/cloud-capacity#provider-support-in-v10). AWS EC2 discovery and provisioning are not implemented.

These paths have different prerequisites and levels of validation. Consult the relevant guide before using a real environment.

## I want to run AkôFlow for the first time

1. [Install AkôFlow Desktop](./installation) and confirm that it connects.
2. [Tour the interface](./guides/interface-tour) to find workflows, plans, runs, and results.
3. If you have the development stack and API credentials, [run the checked-in SimGrid example](./guides/workflows/first-run). It verifies three activities and two transfers.

The SimGrid tutorial currently uses repository files and the API. It is **not** a Desktop-only first-run tutorial. A complete Desktop walkthrough remains to be validated and documented.

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

## I am connecting infrastructure

Choose the guide for the actual target. Provider and runtime support are not interchangeable.

- [SimGrid first run](./guides/workflows/first-run): deterministic local simulation.
- [Kubernetes real execution](./showcase/kubernetes-real-execution): container execution on the checked-in Kind example.
- [HPC and SLURM](./guides/infrastructure/hpc-slurm): login nodes, partitions, shared storage, SSH proxies, and batch execution.
- [Google Cloud](./guides/infrastructure/gcp): service-account credentials, catalog discovery, pricing, and Terraform provisioning.
- [AWS](./guides/infrastructure/aws): S3 and S3-compatible data movement. AkôFlow v1.0 does not discover or provision EC2 capacity.

Review the [cloud support matrix](./guides/infrastructure/cloud-capacity#provider-support-in-v10) before designing a cloud deployment.

## I am automating through the API

Read the [API overview](./reference/api-overview) for the base URL, authentication, content types, asynchronous operations, error envelope, and generated endpoint index. Use the [workflow specification](./internal/workflow-spec) for portable YAML authoring.

The Desktop and HTTP API operate on the same persisted records. The API is preferable for repeatable experiments and integrations; Desktop is preferable for inspecting infrastructure, candidate Gantt charts, live activity state, and plan-versus-observed evidence.

## I want to understand the results

Start with [Core concepts](./concepts) for workflow, environment, plan, run, artifacts, and provenance. A plan describes a proposed execution; a run records the observed one. For implementation details, see [Architecture internals](./modules).

## When something fails

Use [Troubleshooting](./guides/operations/troubleshooting) for daemon readiness, authentication, Docker and BuildKit checks, connection failures, and diagnostic collection. For remote targets, validate credentials and the environment connection before debugging the workflow itself.
