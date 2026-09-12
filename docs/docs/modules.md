---

id: modules
title: AkôFlow components and boundaries
sidebar_label: Components and boundaries
---

import useBaseUrl from '@docusaurus/useBaseUrl';

AkôFlow is a single control-plane daemon with a REST API, a persistent event queue, planning and execution services, and pluggable infrastructure adapters. The Desktop application is the primary client of that API. AkôFlow does **not** deploy a separate Workflow Engine into every environment.

## At a glance

<img src={useBaseUrl('/img/architecture/control-plane-components.svg')} alt="AkôFlow Desktop and API clients call one daemon. The daemon contains API services, a persistent event loop, repositories, planning, execution, cloud lifecycle and data-preparation responsibilities, then uses adapters for local, Kubernetes, SSH or Slurm, and cloud targets." />

The daemon owns orchestration and state. Target environments expose compute, storage, and network capabilities; they do not need an AkôFlow server installed inside each environment.

## Desktop application

The Desktop application and development web UI use the same React interface and REST API. Desktop packages the UI with local daemon lifecycle, configuration, update, and notification support. Its product areas mirror API domains:

- **Workflows** — definitions, immutable versions, activities, dependencies, and planning sessions.
- **Infrastructure** — environments, connections, discovered inventory, machine configurations, execution scopes, network topology, storage, and cloud capacity.
- **Runs** — real, simulated, and interactive executions, activity status, logs, transfers, and planned-versus-observed timing.
- **Artifacts** — executable artifacts, immutable variants, locations, builds, and materializations.
- **Provenance** and **Audit** — scientific lineage and operational actions respectively.
- **Settings** and **Console** — instance configuration, credential references, and supported interactive access.

The UI is a client, not a second implementation of the control plane. Desktop actions call the same API available to automation clients.

## API and services

The HTTP server handles authentication, request validation, and representation. Handlers delegate to services for connection checks, discovery, workflows, planning, execution, storage, transfers, artifact builds, cloud provisioning, console commands, and terminal sessions. Operational and analytics/provenance persistence have distinct responsibilities.

## Persistent event loop

Long-running commands are queued rather than completed inside the initiating HTTP request. The daemon dispatches persistent typed jobs for planning sessions, execution runs, activities, cloud operations, and execution/activity domain events. Queue ownership and retries make work recoverable across interruptions. Clients should observe resource status instead of depending on the current 500 ms polling default.

## Planning

A planning session freezes the workflow version, execution scope, environment versions, resources, topology, activity profiles, deadline, budget, and optional interference model. Registered algorithms generate comparable candidates; built-ins currently include HEFT, PRISM Time, and PRISM Cost.

Selecting a candidate creates or selects a schedule plan; it does not execute the workflow. A plan contains assignments, predicted timing and cost, transfer estimates, and optional cloud lifecycle actions.

## Execution

The execution supervisor consumes a selected plan. It validates the DAG and assignments, prewarms planned cloud capacity, finds dependency-ready activities, prepares executable/workspace data, resolves runtime adapters, starts and inspects handles, records observations, and releases ephemeral capacity. Simulation uses a simulator rather than real adapters. Interactive execution returns while its activity/session remains active.

## Infrastructure and data plane

An **environment** is a managed infrastructure boundary. Published versions contain runtimes, resources, bindings, storage, relations, connections, and capability observations. An **execution scope** combines environment versions with a network topology.

Before start, the data plane can materialize executable artifacts and workspaces. Routes may use an existing location, shared storage, destination pull, source push, a gateway, runtime-local, or direct-runtime transfer. Implemented connectors include the artifact store, local filesystem, rsync/SSH, Kubernetes exec, HTTP, S3-compatible storage, and GCS. A materialization is usable only after digest verification commits it.

## Cloud lifecycle

Cloud support separates catalog/configuration, capacity targets, provisioned instances, and operations. A real run can queue create/start work, bind the resulting runtime allocation, and release it afterward. Cleanup failure is recorded independently and does not rewrite successful scientific computation as failed.

## Provenance and audit

Provenance links workflow versions, plans, runs, activities, data, artifacts, locations, materializations, and transfers. Audit records control-plane actions and their results. They are related but intentionally separate histories.

## Source map

| Concern | Current implementation |
|---|---|
| Daemon composition | `cmd/server/application.go` |
| HTTP routes | `internal/api/httpserver/httpserver.go` |
| Persistent dispatch | `internal/controlplane/eventloop` |
| Plan execution | `internal/controlplane/execution` |
| Runtime contract | `internal/application/ports/execution.go` |
| Domain model | `internal/domain` |
| Runtime adapters | `internal/provider` |
