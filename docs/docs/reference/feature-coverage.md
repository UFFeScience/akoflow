---
id: feature-coverage
title: Desktop and API coverage
sidebar_label: Desktop/API coverage
description: Map every AkôFlow Desktop area to its API family and task documentation.
---

Use this map to find the guide and API family for a Desktop task. Some routes appear only after opening a record or starting an action.

The `/network` and `/network/new` routes exist in Desktop, but the current sidebar and scope detail do not link to them. To register network links through a documented path, use the [execution-scope API procedure](/docs/guides/infrastructure/execution-scopes#using-the-api-1).

## Workflows and execution

| Desktop route | Purpose | API family | Guide |
|---|---|---|---|
| `/workflows` | List versioned workflow definitions | `/workflow-definitions/` | [Workflow definitions](/docs/guides/workflows/definitions) |
| `/workflows/new` | Create or import a workflow | `/workflow-definitions/`, `/workflow-definitions/import/` | [Workflow definitions](/docs/guides/workflows/definitions) |
| `/workflows/:id` | Inspect activities, dependencies, plans, and runs | Workflow, plan, and execution reads | [Workflow definitions](/docs/guides/workflows/definitions) |
| `/workflows/:id/plans/new` | Generate candidates or define a manual plan | `/planning-sessions/`, `/schedule-plans/` | [Plan a workflow](/docs/guides/workflows/planning) |
| `/planning-sessions` | List algorithm comparison sessions | `/planning-sessions/` | [Plan a workflow](/docs/guides/workflows/planning) |
| `/planning-sessions/:id` | Follow algorithms and compare candidates | Planning session and candidate endpoints | [Plan a workflow](/docs/guides/workflows/planning) |
| `/plans` | Aggregate plan index | `/schedule-plans/` | [Plan a workflow](/docs/guides/workflows/planning) |
| `/plans/:id` | Inspect a saved plan | `/schedule-plans/:planId/` | [Plan a workflow](/docs/guides/workflows/planning) |
| `/workflows/:workflowId/plans/:id` | Inspect one plan and its predicted Gantt | `/schedule-plans/:planId/` | [Plan a workflow](/docs/guides/workflows/planning) |
| `/executions` | Filter workflow, standalone, and interactive runs | `/execution-runs/` | [Execute and monitor](/docs/guides/workflows/executions) |
| `/executions/:id` | Inspect a run outside the workflow-specific path | `/execution-runs/:runId/` | [Execute and monitor](/docs/guides/workflows/executions) |
| `/executions/:id/activities/:activityId` | Inspect an activity from that run | Execution context returned with the run detail | [Execute and monitor](/docs/guides/workflows/executions) |
| `/workflows/:workflowId/plans/:planId/executions/:id` | Compare the selected plan with observed execution | `/execution-runs/:runId/` | [Execute and monitor](/docs/guides/workflows/executions) |
| `/workflows/:workflowId/plans/:planId/executions/:id/activities/:activityId` | Inspect one activity attempt | Execution context returned with the run detail | [Execute and monitor](/docs/guides/workflows/executions) |
| `/executions/new` | Start a run from a plan | `POST /execution-runs/` | [Execute and monitor](/docs/guides/workflows/executions) |

## Infrastructure

| Desktop route | Purpose | API family | Guide |
|---|---|---|---|
| `/environments` | List connected and simulated environments | `/environments/` | [Environments](/docs/guides/infrastructure/environments) |
| `/environments/new` | Connect real infrastructure or define simulation infrastructure | Environments, connection tests, SSH keys, Kubernetes tokens | [Environments](/docs/guides/infrastructure/environments) |
| `/environments/:id` | Inspect one environment and its ownership hierarchy | `/environments/:environmentId/` | [Environments](/docs/guides/infrastructure/environments) |
| `/environments/:id/edit` | Replace an environment definition or connection | Environment and connection updates | [Environments](/docs/guides/infrastructure/environments) |
| `/environments/:id/inventory` | Inspect discovered compute, partitions, nodes, and filesystems | Environment discovery and `/resources/` | [Environments](/docs/guides/infrastructure/environments) |
| `/environments/:id/storages` | Browse approved storage roots | `/storages/` | [Storage](/docs/guides/infrastructure/storage) |
| `/resources` | Aggregate compute inventory | `/resources/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/resources/:id` | Inspect capacity, bindings, snapshots, and provisioning | `/resources/:resourceId/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/execution-scopes` | List planning boundaries | `/execution-scopes/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/execution-scopes/new` | Combine environment versions | `POST /execution-scopes/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/execution-scopes/:id` | Inspect a saved scope | `/execution-scopes/:scopeId/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/network` | List network topologies | `/network-topologies/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/network/new` | Create topology metadata and links | `POST /network-topologies/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/network/:id` | Inspect a saved topology | `/network-topologies/:topologyId/` | [Execution scopes](/docs/guides/infrastructure/execution-scopes) |
| `/machine-configurations` | Validate and version configuration playbooks | `/machine-configurations/` | [Machine configurations](/docs/guides/infrastructure/machine-configurations) |
| `/environments/:id/cloud-capacity` | Refresh provider catalog and configure capacity targets | Cloud catalog and capacity-target endpoints | [Cloud capacity](/docs/guides/infrastructure/cloud-capacity) |
| `/environments/:id/provisioning` | List environment provisioning operations | `/cloud-operations/` | [Cloud capacity](/docs/guides/infrastructure/cloud-capacity) |
| `/environments/:id/provisioning/:instanceId` | Follow an instance from its environment | Cloud instance and operation endpoints | [Cloud capacity](/docs/guides/infrastructure/cloud-capacity) |
| `/resources/:resourceId/provisioning/:instanceId` | Follow a resource-owned operation and logs | Cloud instance and operation endpoints | [Cloud capacity](/docs/guides/infrastructure/cloud-capacity) |

## Artifacts, evidence, and operations

| Desktop route | Purpose | API family | Guide |
|---|---|---|---|
| `/artifacts` | List executable artifact definitions | `/artifacts/` | [Build an executable](/docs/guides/data/build-executable) |
| `/artifacts/new` | Register an OCI artifact or start a build | Artifact and build endpoints | [Build an executable](/docs/guides/data/build-executable) |
| `/artifacts/:id` | Inspect versions, locations, builds, and materializations | Artifact detail families | [Artifact locations](/docs/guides/data/artifact-locations) |
| `/artifact-locations` | Compatibility/aggregate location catalog | `/artifact-locations/` | [Artifact locations](/docs/guides/data/artifact-locations) |
| `/materializations` | Compatibility/aggregate materialization catalog | `/artifact-materializations/` | [Artifact locations](/docs/guides/data/artifact-locations) |
| `/data` | Generated scientific data grouped by workflow | Provenance data projections | [Trace a result](/docs/guides/data/provenance) |
| `/provenance` | Query local evidence with read-only SQL | `/provenance/sql/` | [Trace a result](/docs/guides/data/provenance) |
| `/audit` | Inspect recorded connection, discovery, and console events | `/audit-events/` | [Inspect audit events](/docs/guides/data/audit-events) |
| `/console` | Open interactive terminal sessions | `/console-commands/`, `/console-sessions/` | [Interactive console](/docs/guides/operations/interactive-console) |
| `/settings` | Manage identity, appearance, instance archives, and reset | Instance and preference endpoints | [Instance management](/docs/guides/operations/instance-management), [Personal preferences](/docs/guides/operations/personal-preferences) |
| `/settings/ssh-keys` | Generate or import daemon-owned SSH keys | `/ssh-keys/` | [SSH service keys](/docs/guides/operations/credentials-and-ssh) |

## Redirects and compatibility routes

- `/plans/new` redirects to workflow selection because a plan must belong to a workflow.
- `/builds` redirects to the artifact catalog because builds belong to artifacts.
- `/ssh-keys` redirects to `/settings/ssh-keys`.
- Environment-level provisioning is an aggregate index; an individual provisioning operation belongs to its resource.
