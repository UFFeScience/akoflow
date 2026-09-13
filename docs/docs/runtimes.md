---
id: runtimes
title: Runtime adapters
sidebar_label: Runtime adapters
description: How AkôFlow maps planned activities to local, HPC, Kubernetes, cloud, and simulated runtimes.
---

A runtime adapter translates an assigned activity into operations on an execution technology. Runtimes belong to an environment version and connect to resources through bindings. Execution resolves the binding for each planned assignment.

This explanation focuses on runtime adapters. Read [Architecture internals](/docs/modules) for the surrounding services and [Execution control plane](/docs/engine) for how the server uses adapters.

## Runtime model

Each runtime has an ID, driver, and `execution` or `simulation` mode. Its configuration and declared capabilities describe what the driver can do. For example, a batch runtime may declare container and shared-storage support; these declarations do not verify a particular cluster or account. The [environment reference](/docs/reference/environment-yaml) lists the fields.

The portable workflow document does not select a runtime through a top-level YAML `runtime` field. A plan assigns resources; execution resolves their runtime bindings.

Adapters implement `Modes`, `Start`, `Inspect`, and `Stop`. A common handle keeps provider identifiers out of orchestration code.

## Drivers

| Driver | Current role | Modes |
|---|---|---|
| `local` | Process on daemon host | real, interactive |
| `kubernetes` | Jobs plus optional Services/PVCs | real, interactive |
| `ssh` | Direct Docker on an SSH host | real |
| `slurm` | Batch submission or explicit direct target | real |
| `simgrid` | Plan/activity simulation | simulation |
| `cloud` | Capacity resolved to a concrete runtime allocation | real via lifecycle binding |
| `serverless` | Schema value only; no built-in adapter | unavailable |

A driver value in the domain model does not prove that its provider is implemented or configured in a particular instance. The current server has no serverless runtime adapter.

## Local

Local launches the entrypoint as an OS process with arguments, environment, and a run/activity workspace. It records PID, combined stdout/stderr, exit status, and before/after artifact snapshots. Stop sends an interrupt. Use it for development and trusted workloads; it provides no container boundary.

## Kubernetes

Kubernetes creates a Job in the configured namespace and, when required, a Service for activity ports and a prepared workspace PVC. Inspection reads Job/Pod state and logs. Output observation uses the activity filesystem/manifest workflow. Separate connection probing and discovery validate the API and populate cluster inventory before planning.

## SSH direct Docker

The SSH factory currently requires a connection configured for direct Docker. It prepares and snapshots a remote workspace, starts a detached container with resource limits and environment, then uses `docker inspect` and `docker logs`. This is not a generic remote-shell runtime.

## Slurm

Slurm renders an `sbatch` script from activity, resource, and preparation context. HPC partition resources select a partition; HPC machine resources can select a node. SSH-backed submission sends the script on stdin while retaining an audit copy. Inspection reconciles scheduler state, log, sentinel/artifact data, and exit status. A resource with `direct` execution target uses the direct path.

## SimGrid

Simulation uses frozen inventory, profiles, topology, transfer costs, and optional interference data to produce a trace without starting jobs. Participating activities declare the `simulation` capability and simulation definition.

## Cloud

Cloud represents capacity that may not exist at planning time. Assignments target capacity; lifecycle actions create/start an instance and produce a concrete runtime allocation. Configuration, credentials, catalog, capacity targets, instances, and operations are separate resources. Use daemon-managed credential references, never secrets in workflows.

## Connection, discovery, and runtime

1. A **connection** describes how the daemon reaches infrastructure.
2. A **connection check** records reachability and diagnostics.
3. **Discovery** produces versioned resources, storage, relations, bindings, and capabilities.
4. A **runtime** launches and observes assigned activities.

Inventory refresh must not mutate the frozen inputs of an existing planning session.

## Data access

Execution is preceded by preparation. Routes may use an existing verified location, shared storage, destination pull, source push, gateway, runtime-local, or direct-runtime transfer. Implemented connectors include artifact store, filesystem, rsync/SSH, Kubernetes exec, HTTP download, and S3-compatible transfer. Direct `gs://` transfer is unavailable in the current server; the GCS connector returns an error until a deployment supplies a working transfer agent.

## User procedures

To connect infrastructure, use [Create and inspect environments](/docs/guides/infrastructure/environments). For the task flow after registration, follow [Define execution scopes](/docs/guides/infrastructure/execution-scopes), [Plan a workflow](/docs/guides/workflows/planning), and [Execute and monitor a workflow](/docs/guides/workflows/executions). Those guides keep the user steps separate from adapter details here.

## Provider extension boundary

Adding a provider starts with an adapter registered for its driver and mode. External infrastructure also needs a connection check and discovery. Data access may need a transfer route. Test activity start, inspection, stop, failures, and output observation before documenting the provider as supported. Keep these provider details behind the runtime interface so workflow and plan records remain provider-neutral.
