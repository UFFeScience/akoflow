---
id: runtimes
title: Runtime adapters
sidebar_label: Runtime adapters
---

A runtime adapter translates an assigned activity into operations on an execution technology. Runtimes belong to an environment version and connect to resources through bindings. Workflows do not select a runtime through a legacy top-level YAML `runtime` field; a selected plan assigns resources and execution resolves their bindings.

This explanation focuses on runtime adapters. Read [Architecture internals](./modules) for the surrounding services and [Execution control plane](./engine) for how the server uses adapters.

## Runtime model

Each runtime declares `id`, name, driver, `execution` or `simulation` mode, optional role/configuration, and capabilities such as batch, interactive, container, GPU, MPI, shared storage, staging, cancellation, log streaming, and simulation.

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
| `serverless` | Reserved domain capability | depends on registered provider |

A driver value in the domain model does not prove that its provider is configured in a particular instance.

## Local

Local launches the entrypoint as an OS process with arguments, environment, and a run/activity workspace. It records PID, combined stdout/stderr, exit status, and before/after artifact snapshots. Stop sends an interrupt. Use it for development and trusted workloads; it provides no container boundary.

## Kubernetes

Kubernetes creates a Job in the configured namespace and, when required, a Service for activity ports and a prepared workspace PVC. Inspection reads Job/Pod state and logs. Output observation uses the activity filesystem/manifest workflow. Separate connection probing and discovery validate the API and populate cluster inventory before planning.

## SSH direct Docker

The SSH factory currently requires a connection configured for direct Docker. It prepares and snapshots a remote workspace, starts a detached container with resource limits and environment, then uses `docker inspect` and `docker logs`. This is not a generic remote-shell runtime.

## Slurm

Slurm renders an `sbatch` script from activity, resource, and preparation context. HPC partition resources select a partition; HPC machine resources can select a node. SSH-backed submission sends the script on stdin while retaining an audit copy. Inspection reconciles scheduler state, log, sentinel/artifact data, and exit status. A resource with `direct` execution target uses the direct path.

## SimGrid

Simulation is a mode, not a fake infrastructure connection. It uses frozen inventory, profiles, topology, transfer costs, and optional interference data to produce a trace without starting jobs. Participating activities declare the `simulation` capability and simulation definition.

## Cloud

Cloud represents capacity that may not exist at planning time. Assignments target capacity; lifecycle actions create/start an instance and produce a concrete runtime allocation. Configuration, credentials, catalog, capacity targets, instances, and operations are separate resources. Use daemon-managed credential references, never secrets in workflows.

## Connection, discovery, and runtime

1. A **connection** describes how the daemon reaches infrastructure.
2. A **connection check** records reachability and diagnostics.
3. **Discovery** produces versioned resources, storage, relations, bindings, and capabilities.
4. A **runtime** launches and observes assigned activities.

Inventory refresh must not mutate the frozen inputs of an existing planning session.

## Data access

Execution is preceded by preparation. Routes may use an existing verified location, shared storage, destination pull, source push, gateway, runtime-local, or direct-runtime transfer. Implementations include artifact store, filesystem, rsync/SSH, Kubernetes exec, HTTP, S3-compatible storage, and GCS. An adapter must reject an uncommitted preparation gate.

## Choose a target

In Desktop:

1. create an environment and connection;
2. validate it and run discovery;
3. review resources, bindings, storage, and capabilities in the inventory;
4. include the published version in an execution scope;
5. plan the workflow and inspect candidate assignments;
6. select a plan and start the required execution mode.

API clients perform the equivalent environment, check, discovery, scope, planning, selection, and execution operations. Use the generated API Reference for exact current routes.

## Provider extension boundary

A complete provider generally needs an adapter, resolver/factory registration, probing and discovery for external infrastructure, endpoint/transfer integration, capability declarations, and tests for start, inspect, stop, failures, and artifact observation. Provider behavior stays behind ports; workflow, planning, and execution domain objects remain provider-neutral.
