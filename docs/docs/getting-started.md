---
id: getting-started
title: Choose where to start
sidebar_label: Getting started
slug: /getting-started
description: Choose the shortest AkôFlow documentation path for installation, a first run, operations, concepts, or API integration.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

# Choose where to start

AkôFlow plans and executes scientific workflow DAGs on simulated or connected infrastructure, then preserves the plan, observed execution, data movement, artifacts, audit events, and provenance. This page is a map of the documentation; it does not teach an individual workflow.

You do not need prior AkôFlow experience. Choose the path that matches what you want to accomplish.

## I want to run AkôFlow for the first time

1. [Install AkôFlow](./installation) and verify that its daemon is available.
2. [Run the first simulated workflow](./guides/workflows/first-run). The tutorial uses checked-in files, requires no cluster or cloud account, and ends with concrete activity and transfer checks.
3. Use the [interface tour](./guides/interface-tour) when you want to learn where the same records appear in Desktop.

Start with the simulation even if your eventual target is Kubernetes or HPC. It separates installation problems from credentials, network access, scheduler policy, and remote storage.

## Continue after installation

Follow [installation result checks](./installation#4-installation-result), then
[register HPC / SLURM](./tutorials/register-hpc) or
[connect Google Cloud](./tutorials/connect-cloud). Each tutorial includes the
actual connection form, an API path, and expected results. For automation, start
with [API connection setup](./tutorials/api-access).

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

## I need to understand the model first

Read [Core concepts](./concepts) for the vocabulary and record relationships. Continue to [Engine](./engine) for control-plane behavior and [Runtimes](./runtimes) for execution-provider boundaries.

The central lifecycle is shown below.

<img src={useBaseUrl('/img/architecture/lifecycle-overview.svg')} alt="AkôFlow lifecycle: an infrastructure boundary and workflow version produce candidate plans; one selected plan produces an execution run and observed evidence." />

A plan is not an execution. It predicts an assignment within a frozen workflow and infrastructure boundary. A run records what happened when that plan was dispatched.

## When something fails

Use [Troubleshooting](./guides/operations/troubleshooting) for daemon readiness, authentication, Docker and BuildKit checks, connection failures, and diagnostic collection. For remote targets, validate credentials and the environment connection before debugging the workflow itself.
