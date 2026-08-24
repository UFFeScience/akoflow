# Akoflow Agent Guide

## Core Shape

- Go project with CLI and server binaries.
- `cmd/server` is the explicit composition root. Do not introduce global service locators.
- Domain packages contain definitions only and cannot import infrastructure.
- Application services coordinate use cases through small ports.
- SQL repositories own complete aggregates and share the process database handle.
- Runtime adapters implement `ports.RuntimeAdapter` and are registered by runtime ID and execution mode.
- The persistent event loop is the asynchronous command transport and source of retry/lease semantics.

## Primary Flow

1. API validates and persists a command in `queue_jobs`.
2. The event loop leases the command and invokes its handler.
3. `ExecutionSupervisor` creates the run and follows the frozen schedule plan.
4. `ActivityExecutionService` resolves the adapter and starts an activity.
5. The adapter returns a runtime-independent `ActivityHandle`.
6. The supervisor inspects handles, persists task observations and completes or fails the run.

Simulation follows the same request and trace contracts. Interactive runs keep the execution run open and expose endpoints through the activity handle.

## Important Invariants

- Activity definition, schedule prediction and execution observation are different records.
- API handlers never import SQL repositories as domain types.
- No one-directory-per-method services. Prefer cohesive services with explicit dependencies.
- No package globals for repositories, connectors, queues or runtime registries.
- Adapters cannot update workflow or plan state directly.
- Provider IDs such as PID, Kubernetes Job and Slurm Job exist only in `ActivityHandle.ExternalID`.
- Queue handlers must be idempotent because leases can be retried.
- A repository operation that changes one aggregate must use one transaction.
- Real, simulation and interactive execution are modes, not separate activity models.

## Provider Packages

- `internal/provider/local`
- `internal/provider/kubernetes`
- `internal/provider/slurm`
- `internal/provider/simgrid`

New runtimes implement the same adapter contract and register their supported modes. Do not add provider conditionals to application services.
