---
id: feature-coverage
title: Desktop and API coverage
sidebar_label: Desktop/API coverage
description: Map every AkôFlow Desktop area to its API family and task documentation.
---

This matrix is maintained as the coverage checklist for the public documentation. A route may be hidden from the primary sidebar when it is a child page, compatibility redirect, or contextual action.

The `/network` and `/network/new` routes exist in Desktop, but the current sidebar and scope detail do not link to them. To register network links through a documented path, use the [execution-scope API procedure](../guides/infrastructure/execution-scopes#using-the-api-1).

## Workflows and execution

| Desktop route | Purpose | API family | Guide |
|---|---|---|---|
| `/workflows` | List versioned workflow definitions | `/workflow-definitions/` | [Workflow definitions](../guides/workflows/definitions) |
| `/workflows/new` | Create or import a workflow | `/workflow-definitions/`, `/workflow-definitions/import/` | [Workflow definitions](../guides/workflows/definitions) |
| `/workflows/:id` | Inspect activities, dependencies, plans, and runs | Workflow, plan, and execution reads | [Workflow definitions](../guides/workflows/definitions) |
| `/workflows/:id/plans/new` | Generate candidates or define a manual plan | `/planning-sessions/`, `/schedule-plans/` | [Plan a workflow](../guides/workflows/planning) |
| `/planning-sessions` | List algorithm comparison sessions | `/planning-sessions/` | [Plan a workflow](../guides/workflows/planning) |
| `/planning-sessions/:id` | Follow algorithms and compare candidates | Planning session and candidate endpoints | [Plan a workflow](../guides/workflows/planning) |
| `/plans` | Aggregate plan index | `/schedule-plans/` | [Plan a workflow](../guides/workflows/planning) |
| `/workflows/:workflowId/plans/:id` | Inspect one plan and its predicted Gantt | `/schedule-plans/:planId/` | [Plan a workflow](../guides/workflows/planning) |
| `/executions` | Filter workflow, standalone, and interactive runs | `/execution-runs/` | [Execute and monitor](../guides/workflows/executions) |
| `/workflows/:workflowId/plans/:planId/executions/:id` | Compare the selected plan with observed execution | `/execution-runs/:runId/` | [Execute and monitor](../guides/workflows/executions) |
| `/workflows/:workflowId/plans/:planId/executions/:id/activities/:activityId` | Inspect one activity attempt | Execution context returned with the run detail | [Execute and monitor](../guides/workflows/executions) |
| `/executions/new` | Start a run from a plan | `POST /execution-runs/` | [Execute and monitor](../guides/workflows/executions) |

## Infrastructure

| Desktop route | Purpose | API family | Guide |
|---|---|---|---|
| `/environments` | List connected and simulated environments | `/environments/` | [Environments](../guides/infrastructure/environments) |
| `/environments/new` | Connect real infrastructure or define simulation infrastructure | Environments, connection tests, SSH keys, Kubernetes tokens | [Environments](../guides/infrastructure/environments) |
| `/environments/:id` | Inspect one environment and its ownership hierarchy | `/environments/:environmentId/` | [Environments](../guides/infrastructure/environments) |
| `/environments/:id/edit` | Replace an environment definition or connection | Environment and connection updates | [Environments](../guides/infrastructure/environments) |
| `/environments/:id/inventory` | Inspect discovered compute, partitions, nodes, and filesystems | Environment discovery and `/resources/` | [Environments](../guides/infrastructure/environments) |
| `/environments/:id/storages` | Browse approved storage roots | `/storages/` | [Storage](../guides/infrastructure/storage) |
| `/resources` | Aggregate compute inventory | `/resources/` | [Execution scopes](../guides/infrastructure/execution-scopes) |
| `/resources/:id` | Inspect capacity, bindings, snapshots, and provisioning | `/resources/:resourceId/` | [Execution scopes](../guides/infrastructure/execution-scopes) |
| `/execution-scopes` | List planning boundaries | `/execution-scopes/` | [Execution scopes](../guides/infrastructure/execution-scopes) |
| `/execution-scopes/new` | Combine environment versions | `POST /execution-scopes/` | [Execution scopes](../guides/infrastructure/execution-scopes) |
| `/network` | List network topologies | `/network-topologies/` | [Execution scopes](../guides/infrastructure/execution-scopes) |
| `/network/new` | Create topology metadata and links | `POST /network-topologies/` | [Execution scopes](../guides/infrastructure/execution-scopes) |
| `/machine-configurations` | Validate and version configuration playbooks | `/machine-configurations/` | [Cloud capacity](../guides/infrastructure/cloud-capacity) |
| `/environments/:id/cloud-capacity` | Refresh provider catalog and configure capacity targets | Cloud catalog and capacity-target endpoints | [Cloud capacity](../guides/infrastructure/cloud-capacity) |
| `/environments/:id/provisioning` | List environment provisioning operations | `/cloud-operations/` | [Cloud capacity](../guides/infrastructure/cloud-capacity) |
| `/resources/:resourceId/provisioning/:instanceId` | Follow a resource-owned operation and logs | Cloud instance and operation endpoints | [Cloud capacity](../guides/infrastructure/cloud-capacity) |

## Artifacts, evidence, and operations

| Desktop route | Purpose | API family | Guide |
|---|---|---|---|
| `/artifacts` | List executable artifact definitions | `/artifacts/` | [Build an executable](../guides/data/build-executable) |
| `/artifacts/new` | Register an OCI artifact or start a build | Artifact and build endpoints | [Build an executable](../guides/data/build-executable) |
| `/artifacts/:id` | Inspect versions, locations, builds, and materializations | Artifact detail families | [Artifact locations](../guides/data/artifact-locations) |
| `/artifact-locations` | Compatibility/aggregate location catalog | `/artifact-locations/` | [Artifact locations](../guides/data/artifact-locations) |
| `/materializations` | Compatibility/aggregate materialization catalog | `/artifact-materializations/` | [Artifact locations](../guides/data/artifact-locations) |
| `/data` | Generated scientific data grouped by workflow | Provenance data projections | [Trace a result](../guides/data/provenance) |
| `/provenance` | Explore entities, SQL, and lineage | `/provenance/` | [Trace a result](../guides/data/provenance) |
| `/audit` | Search operational history | `/audit-events/` | [Inspect audit events](../guides/data/audit-events) |
| `/console` | Open interactive terminal sessions | `/console-commands/`, `/console-sessions/` | [Interactive console](../guides/operations/interactive-console) |
| `/settings` | Manage identity, appearance, instance archives, and reset | Instance and preference endpoints | [Instance management](../guides/operations/instance-management), [Personal preferences](../guides/operations/personal-preferences) |
| `/settings/ssh-keys` | Generate or import daemon-owned SSH keys | `/ssh-keys/` | [SSH service keys](../guides/operations/credentials-and-ssh) |

## Redirects and compatibility routes

- `/plans/new` redirects to workflow selection because a plan must belong to a workflow.
- `/builds` redirects to the artifact catalog because builds belong to artifacts.
- `/ssh-keys` redirects to `/settings/ssh-keys`.
- Environment-level provisioning is an aggregate index; an individual provisioning operation belongs to its resource.

## Verification rule

When a Desktop route or daemon endpoint is added, the same change must update or regenerate this documentation:

1. Endpoint pages regenerate automatically from the Go router.
2. The relevant domain guide explains the user task and payload semantics.
3. This matrix records where the operation appears in Desktop.
4. A screenshot is added only when the spatial interface conveys information that text does not.
