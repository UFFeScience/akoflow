---

id: modules
title: Architecture internals
sidebar_label: Architecture internals
description: How the AkôFlow server coordinates planning, execution, persistence, and adapters.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

The AkôFlow server exposes one REST API and coordinates planning, execution, and saved state. Desktop uses that API. Runtime adapters connect the server to local, cluster, and cloud execution technologies.

## At a glance

<img src={useBaseUrl('/img/architecture/control-plane-components.svg')} alt="AkôFlow Desktop and API clients call one daemon. The daemon contains API services, a persistent event loop, repositories, planning, execution, cloud lifecycle and data-preparation responsibilities, then uses adapters for local, Kubernetes, SSH or Slurm, and cloud targets." />

The daemon owns orchestration and state. Target environments expose compute, storage, and network capabilities; they do not need an AkôFlow server installed inside each environment.

## Desktop application

The Desktop application and development web UI use the same React interface and REST API. Desktop packages the UI with local daemon lifecycle, configuration, update, and notification support. Its product areas mirror API domains:

- **Workflows** — definitions, immutable versions, activities, dependencies, and planning sessions.
- **Infrastructure** — environments, connections, discovered inventory, machine configurations, execution scopes, network topology, storage, and cloud capacity.
- **Runs** — real, simulated, and interactive executions, activity status, logs, transfers, and planned-versus-observed timing.
- **Artifacts** — executable artifacts, immutable variants, locations, builds, and materializations.
- **Provenance** — scientific lineage; **Audit** — recorded connection, discovery, and console actions.
- **Settings** and **Console** — instance configuration, credential references, and supported interactive access.

The UI is a client, not a second implementation of the control plane. Desktop actions call the same API available to automation clients.

## API and services

The HTTP server authenticates and validates requests, then calls the service responsible for the task. Planning, execution, infrastructure checks, and data preparation have separate services. Operational state and provenance data also have separate persistence paths; the [source map](#source-map) points to their entry points.

## Persistent event loop

Long-running commands are queued rather than completed inside the initiating HTTP request. The daemon dispatches persistent typed jobs for planning sessions, execution runs, activities, cloud operations, and execution/activity domain events. An expired queue lease can return a job to the pending state; this does not resume a workflow run already started by the supervisor. Clients observe the status of the requested operation.

## Planning

A planning session freezes the selected workflow, execution scope, topology, resources, and planning constraints. Built-in algorithms include HEFT, PRISM Time, and PRISM Cost. They use the same session inputs, but their predictions come from different evaluation models; see [PRISM and HEFT](/docs/explanations/prism-and-heft) before comparing them.

Selecting a candidate creates or selects a schedule plan; it does not execute the workflow. A plan contains assignments, predicted timing and cost, transfer estimates, and optional cloud lifecycle actions.

## Execution

The execution supervisor checks the selected plan and starts activities when their dependencies are ready. It prepares their executable and workspace data, selects a runtime adapter, then follows each activity through completion. When a real plan needs cloud capacity, it can prepare that capacity before dispatch and release it afterward. Simulation uses a simulator instead of real adapters. An interactive request returns while its session remains active.

## Infrastructure and data plane

An **environment** is a managed infrastructure boundary. Published versions contain runtimes, resources, bindings, storage, relations, connections, and capability observations. An **execution scope** selects environment versions; a planning session also chooses a network topology.

Before start, the data plane can prepare executable artifacts and workspaces. Implemented transfer paths include the artifact store, local filesystem, rsync/SSH, Kubernetes exec, HTTP download, and an S3-compatible connector. The current GCS connector rejects direct `gs://` transfers; a deployment needs another supported route or its own transfer agent. A prepared artifact is usable only after digest verification.

## Cloud lifecycle

Cloud support separates catalog/configuration, capacity targets, provisioned instances, and operations. A real run can queue create/start work, bind the resulting runtime allocation, and release it afterward. Cleanup failure is recorded independently and does not rewrite successful scientific computation as failed.

## Provenance and audit

Provenance links workflow versions, plans, runs, activities, data, artifacts, locations, materializations, and transfers. Audit currently records connection health, resource discovery, and console actions and their outcomes. It does not record every control-plane change. These are separate histories.

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
