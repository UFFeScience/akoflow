# Akoflow HPC Execution Flow for AI

Purpose: explain how Akoflow executes workflows on HPC infrastructure through Slurm.

Akoflow is the workflow engine core of the project. In the HPC path, it turns workflow activities into remote jobs executed on a cluster managed by Slurm.

## Supported Execution Target

- HPC cluster with Slurm

## Main Idea

- The client submits the workflow.
- The server persists the workflow and activities.
- The orchestrator schedules pending activities.
- The worker dispatches the activity to the runtime selected by its runtime id.
- The HPC runtime service builds a Singularity command and wraps it in an sbatch submission.
- The Slurm provider is the sbatch template plus the Slurm submission command.

## Shared Dispatch Path

1. `run_activity_in_cluster_service` loads the activity and workflow.
2. It reads the activity runtime id.
3. `runtimes.GetRuntimeInstance(runtimeId)` returns `HpcRuntime` when the runtime is HPC.
4. `HpcRuntime.ApplyJob` delegates to `HPCRuntimeService.ApplyJob`.

## Worker To HPC Flow

1. `pkg/server/engine/worker/worker.go` receives an activity id from the channel.
2. `run_activity_in_cluster_service.Run(...)` loads workflow and activity data.
3. `pkg/server/runtimes/runtime.go` resolves the HPC runtime wrapper.
4. `pkg/server/runtimes/hpc_runtime/hpc_runtime.go` forwards execution to `HPCRuntimeService`.
5. `pkg/server/runtimes/hpc_runtime/hpc_runtime_service/hpc_runtime_service.go` prepares the remote job.

## HPC Runtime Service

### Main classes

- `HpcRuntime`
- `HPCRuntimeService`
- `MakeSingularityActivityService`
- `MakeSBatchHPCRuntimeActivityService`
- `connector_hpc`
- `runtime_repository`

### ApplyJob Flow

1. The service loads `WorkflowActivities` and `Workflow` from repositories.
2. It blocks duplicate execution if the activity is already running.
3. If the workflow is still in created state, it prepares the workflow in the HPC runtime.
4. It loads the runtime metadata from `runtime_repository`.
5. It builds the Singularity command for the activity.
6. It wraps the Singularity command inside an sbatch command.
7. It checks VPN connectivity through `connector_hpc.IsVPNConnected()`.
8. It submits the remote command with `connector_hpc.RunCommandWithOutputRemote(...)`.
9. It extracts the Slurm job id from the output.
10. It updates workflow status, activity status and activity process id.

## Slurm Provider

The Slurm provider is implemented by the sbatch template flow.

### Template Source

- `pkg/server/engine/scripts/default-slurm.sbatch` is the fallback template.
- A runtime can also provide a custom `SBATCHTEMPLATE` metadata value.
- The custom value is stored as base64 and decoded before use.

### Template Builder

`pkg/server/runtimes/hpc_runtime/hpc_runtime_service/make_sbatch_hpc_activity_service.go` performs the rendering.

It injects:

- job name
- output path
- error path
- time limit
- partition or queue
- task count
- node count
- GPUs
- CPUs per GPU
- memory limit
- the wrapped Singularity command

### Final Command Shape

1. The sbatch template is filled with runtime metadata.
2. The service appends an `AKOFLOW_JOB_FINISHED` marker file write.
3. The template is base64 encoded.
4. The command is decoded and piped into `sbatch`.
5. The resulting command is base64 encoded again and executed locally in the controller shell.
6. The remote Slurm submission happens through SSH in the HPC connector.

## Connector HPC

`pkg/server/connector/connector_hpc/connector_hpc.go` is the external I/O layer for the HPC path.

### Responsibilities

- Build remote SSH commands.
- Support key-based or password-based authentication.
- Execute remote commands and capture output.
- Check VPN availability before remote submission.
- Execute multiple remote commands when synchronizing volumes.

### Relation To Runtime Metadata

- `USER` and `HOST_CLUSTER` define the remote SSH target.
- `SSHKEYPRIVK`, `SSHKEYPUBLK` and `SSHCONFIG` enable key-based access.
- `PASSWORD` enables password-based access.
- `MOUNT_PATH` tells the runtime where files must exist on the remote side.
- `SBATCHTEMPLATE` overrides the default Slurm template.

## Volume Synchronization

`HPCRuntimeService.syncWorkflowVolumes(...)` is the persistence bridge between local storage and the HPC cluster.

### Flow

1. Read workflow volumes from the workflow entity.
2. Create the remote destination directory.
3. Rsync local volume contents to the remote cluster.
4. Rsync the remote results back to the local volume path.

### Why It Matters

- Generated files must survive job completion.
- The mount path on the HPC side must match the expected runtime path.
- If the mount path is wrong, the job may complete but the artifacts will not be found later.

## Verification Flow

`HPCRuntimeService.VerifyActivitiesWasFinished(...)` checks job progress after submission.

1. Iterate through workflow activities.
2. Only inspect activities that belong to the current HPC runtime name.
3. Load the database state for the activity.
4. Skip finished or not-yet-started activities.
5. Load the runtime metadata from the repository.
6. Read the completion marker file from the remote mount path.
7. Query the Slurm job state through the remote connector.
8. Mark the activity as finished when the job is completed or failed.
9. Sync workflow volumes back when the job is done.

## State Handling

- Created activity: no execution yet.
- Running activity: submitted to Slurm and waiting or executing.
- Finished activity: the completion marker was observed or the Slurm state indicates end of execution.

## Relationships Between Classes

- `Worker` pulls work from the channel.
- `RunActivityInClusterService` resolves the runtime.
- `HpcRuntime` adapts the common runtime interface.
- `HPCRuntimeService` owns the HPC business logic.
- `MakeSBatchHPCRuntimeActivityService` renders the Slurm submission.
- `connector_hpc` sends commands over SSH and VPN.
- `runtime_repository` provides the remote metadata used to build the submission.
- `activity_repository` and `workflow_repository` store the lifecycle state.

## AI Notes

- This is not a generic batch runner.
- The execution is a three-step composition: local controller command, SSH remote command, Slurm submission.
- The Singularity command is embedded inside the sbatch template.
- Debugging should start with the runtime metadata, the sbatch template, and the remote connector.
- If files are missing after completion, inspect the mount path and rsync synchronization first.