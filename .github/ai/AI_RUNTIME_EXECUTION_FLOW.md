# Akoflow Runtime Execution Flow for AI

Purpose: compact reference for how each runtime works and how the classes relate.

Akoflow is the workflow engine core of the project. It takes workflow definitions and turns them into executions on supported backends.

Supported backends:

- Kubernetes clusters
- HPC environments orchestrated with Slurm

## Shared Dispatch Path

`RunActivityInClusterService` is the common entrypoint for activity execution.

This service is part of the execution core: it decides where the activity runs and forwards control to the correct runtime implementation.

1. Load `WorkflowActivities` and `Workflow` from repositories.
2. Read `runtimeId` from the activity.
3. Call `runtimes.GetRuntimeInstance(runtimeId)`.
4. Execute `ApplyJob(workflowID, activityID)` on the selected runtime wrapper.

## Runtime Pattern

- Wrapper package: `pkg/server/runtimes/<runtime>`.
- Service package: `pkg/server/runtimes/<runtime>/<runtime>_service`.
- Wrapper: implements the common interface and delegates.
- Service: builds commands, calls connectors, updates repositories, collects logs and metrics.

## Local Runtime

### Main classes

- `LocalRuntime`
- `LocalRuntimeService`
- `connector_local`

### Execution

1. `LocalRuntime.ApplyJob` delegates to `LocalRuntimeService.ApplyJob`.
2. The service loads activity and workflow records.
3. It builds a local shell command from `wfa.Run`.
4. It appends a completion marker file and redirects stdout/stderr to the mount path.
5. `connector_local.RunCommand` executes the command.
6. Workflow and activity status are set to running.
7. The process ID is saved in the activity record.

### Verification

- `VerifyActivitiesWasFinished` checks for the completion marker file.
- If the job is still running, it extracts metrics and logs from the command output.
- Metrics go to `metrics_repository` and logs go to `logs_repository`.

## Singularity Runtime

### Main classes

- `SingularityRuntime`
- `SingularityRuntimeService`
- `MakeSingularityActivityService`
- `connector_singularity`

### Execution

1. `SingularityRuntime.ApplyJob` delegates to `SingularityRuntimeService.ApplyJob`.
2. The service loads activity and workflow records.
3. `MakeSingularityActivityService` creates the `singularity exec` command.
4. The command binds the workflow mount path and sets the container working directory.
5. `connector_singularity.RunCommand` submits the command.
6. Workflow and activity status are updated.
7. The process ID is saved.

### Verification

- `VerifyActivitiesWasFinished` runs the Singularity monitor script.
- The output is parsed to detect completion.
- Metrics and logs are extracted and persisted when the job is still active.

## HPC Runtime

### Main classes

- `HpcRuntime`
- `HPCRuntimeService`
- `MakeSingularityActivityService`
- `MakeSBatchHPCRuntimeActivityService`
- `connector_hpc`
- `runtime_repository`

### See Also

- [`HPC_EXECUTION.md`](HPC_EXECUTION.md) contains the full AI-focused HPC execution map.

### Execution

1. `HpcRuntime.ApplyJob` delegates to `HPCRuntimeService.ApplyJob`.
2. The service loads activity and workflow records.
3. It may prepare workflow data and environment for the first activity.
4. Runtime metadata is loaded from `runtime_repository`.
5. `MakeSingularityActivityService` builds the Singularity command used on the cluster.
6. `MakeSBatchHPCRuntimeActivityService` wraps that command in an sbatch submission.
7. `connector_hpc` checks VPN connectivity.
8. `connector_hpc` submits the remote command.
9. The returned job ID is stored as the process ID.
10. Workflow and activity status are updated to running.

### Volume Sync

- `syncWorkflowVolumes` prepares remote directories.
- It copies workflow data to the cluster and later pulls results back.
- This is the critical relation between workflow volumes and remote execution.

### Verification

- `VerifyActivitiesWasFinished` checks each running activity.
- It reads remote completion state and job status.
- If the job is complete or failed, it syncs volumes back and closes the activity.

## Kubernetes Runtime

### Main classes

- `KubernetesRuntime`
- `KubernetesRuntimeService`
- `ModeRunActivityService`
- `MonitorVerifyActivityWasFinishedService`
- `MonitorGetLogsActivityService`
- `MonitorGetMetricsActivityService`

### Execution

1. `KubernetesRuntime.ApplyJob` delegates to `KubernetesRuntimeService.ApplyJob`.
2. The service loads activity and workflow records.
3. It resolves the workflow mode.
4. The mode-specific service prepares Kubernetes resources.
5. The job is applied in the cluster.

### Monitoring

- `VerifyActivitiesWasFinished` delegates to a monitor service.
- `GetLogs` and `GetMetrics` delegate to dedicated monitor services.
- `HealthCheck` validates the runtime and discovers nodes.

## Docker Runtime

- Docker exists in the runtime factory.
- It behaves like a placeholder compared to the other runtimes.

## Relationship Summary

- Entities define workflow state.
- Repositories store workflow state.
- Connectors perform external I/O.
- Runtime services coordinate everything.
- Runtime wrappers only adapt to the common interface.

## AI Guidance

- When tracing a bug, start with the runtime wrapper, then the runtime service, then the connector and repositories.
- For persistence bugs, inspect mount path handling first.
- For execution bugs, inspect runtime selection and activity state transitions first.