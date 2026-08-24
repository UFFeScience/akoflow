# Akoflow AI Architecture Map

Purpose: machine-readable notes for AI agents working on this repository.

## System Shape

- Akoflow is the workflow engine core of the system.
- Its job is to coordinate workflow execution across supported infrastructures.
- Supported execution targets are Kubernetes clusters and HPC environments managed with Slurm.
- Language: Go.
- Main split: client binary and server binary.
- Core pattern: layered architecture with config, handlers, services, runtimes, connectors, repositories and entities.

## Entry Points

- `cmd/akoflow/main.go`: starts the CLI.
- `cmd/server/main.go`: owns process signals and application lifetime only.
- `cmd/server/application.go`: composes and runs the server application.
- `cmd/server/persistence.go`: bootstraps the canonical database and repositories.
- `cmd/server/runtimes.go`: composes runtime adapters and external clients.
- `cmd/server/events.go`: composes the persistent event loop and execution handlers.
- `cmd/server/processes.go`: starts background processes such as Kubernetes history cleanup.
- `cmd/server/api.go`: composes the HTTP API dependencies.

## High-Level Flow

1. Client reads a workflow file and submits it to the server.
2. HTTP handlers receive the request and call application services.
3. Services persist workflow and activity state through repositories.
4. The orchestrator loads pending workflows and schedules activities.
5. The worker consumes activity IDs and dispatches them to the selected runtime.
6. The runtime wrapper delegates to the runtime service.
7. The runtime service uses connectors and repositories to execute, monitor and finalize the job.

## Layer Responsibilities

### Config

- `pkg/server/config/app_container.go` wires repositories, connectors, logger, helpers and renderer.
- `config.App()` is the global dependency source.

### HTTP

- `pkg/server/engine/httpserver` exposes the HTTP server.
- `pkg/server/engine/httpserver/handlers` contains the route handlers.
- Handlers should only coordinate input/output and call services.

### Services

- `pkg/server/services` contains business logic.
- This layer orchestrates workflow creation, scheduling, monitoring, metrics, logs and runtime execution.

### Repositories

- `pkg/server/database/repository` persists workflow, activity, runtime, schedule, storage, logs and metrics data.
- SQLite is the default persistence backend.

### Entities

- `pkg/server/entities` holds the domain models.
- Workflow and activity entities are the main data carriers across layers.

### Runtimes

- `pkg/server/runtimes/runtime.go` defines the common runtime interface and the runtime factory.
- Supported runtime names: `local`, `singularity`, `k8s`, `hpc`, `docker`.

### Connectors

- `pkg/server/connector` contains adapters for local shell execution, Singularity, Kubernetes and HPC integration.

## Composition Model

- Runtime wrappers are thin adapters.
- Runtime services contain the real execution logic.
- Runtime services depend on repositories for state and connectors for external execution.
- Entities flow through the system as the shared domain model.

## Relationship Rules

- Handlers call services, not repositories.
- Services may call repositories and connectors.
- Runtimes are selected from workflow/activity metadata.
- Runtime wrappers should not contain business logic.
- Connectors should isolate external APIs and shell commands.

## Important Paths

- Client entry: `cmd/client/main.go`
- Server entry: `cmd/server/main.go`
- App container: `pkg/server/config/app_container.go`
- Runtime factory: `pkg/server/runtimes/runtime.go`
- Activity dispatch: `pkg/server/services/run_activity_in_cluster_service`

## Persistence Note

- File persistence depends on the runtime mount path being consistent between host and execution environment.
- This is critical for Singularity and HPC, where generated artifacts must remain visible after job completion.

## What AI Should Infer

- The project is an execution engine, not just an API service.
- The server translates workflow definitions into jobs for the target runtime environment.
- The codebase is not monolithic; it is a composition of small services.
- Runtime behavior is delegated, not centralized.
- The app container is the main dependency graph.
- Most bugs around execution come from mount paths, runtime selection, or repository state transitions.
