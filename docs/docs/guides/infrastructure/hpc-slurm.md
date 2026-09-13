---
title: Connect an HPC and SLURM cluster
description: Configure SSH access, discover SLURM resources, and prepare a small site-approved batch validation.
---

# Connect an HPC and SLURM cluster

This guide is for an HPC operator or researcher with an approved SLURM account. AkôFlow connects to the **login node** over SSH and submits workflow activities with `sbatch`. For a guided first registration, start with the [connection tutorial](../../tutorials/register-hpc).

Use a SLURM environment for work governed by partitions, accounts, QoS, and node allocation. Keep ordinary batch work off the login node. To check the local batch-submission path without a cluster, use the [SLURM batch fixture](../../showcase/slurm-local-fixture); it does not verify SSH, allocation, accounting, or site policy. For an infrastructure simulation, use [SimGrid](./simgrid).

The YAML blocks below are excerpts for discussion. Use the linked versioned files as a starting catalog, then supply your site's connection, paths, permissions, and scheduler settings. An institutional cluster run has not been verified by this documentation.

## Prerequisites

- An account authorized for non-interactive SSH to the login node and for `sbatch`, `squeue`, `sacct`, and `scancel` under the intended project/account and partition.
- An SSH public key authorized on every necessary hop, including a bastion when one is required.
- The cluster's host-key fingerprint, login hostname, SSH port, partition name, account/QoS constraints, and a compute-visible workspace path.
- A container runtime approved by the site when workflow activities require one. The checked-in example uses Apptainer but does not install or configure it.
- A test allocation approved by the site. Do not use the first validation run to request a large partition or a long wall time.

## 1. Create the SSH credential and proxy-aware connection

Create or import a service key using [Manage SSH service keys](../operations/credentials-and-ssh), then authorize its public key on the login node and any gateway. Store the returned `credentialRef` in the connection; never paste the private key into an environment YAML.

For a remote cluster, use `type: ssh`. Put the SSH port in `configuration.port` and the login host name in `endpoint`; AkôFlow uses this connection for health checks, discovery, runs, artifacts, and the terminal. Set the fields below in the [HPC registration template](../../tutorials/register-hpc), using the credential reference returned by key registration.

```yaml
connections:
  - id: research-hpc-connection
    environmentId: research-hpc
    name: Research HPC login node
    type: ssh
    endpoint: login.example.org
    username: researcher
    credentialRef: REPLACE_WITH_RETURNED_CREDENTIAL_REF
    configuration:
      port: 22
      hostKeyAlias: research-hpc-login
      knownHostsFile: storage/credentials/ssh/known_hosts
      proxyCommand: ssh gateway.example.org -W login.example.org:22
      scriptDirectory: /scratch/researcher/akoflow/scripts
```

`proxyCommand` is passed to SSH-based paths that use this connection. If the site requires a jump host, configure and test a complete SSH proxy command from the **server host**, not only from Desktop. AkôFlow records trusted host keys in the configured known-hosts file; keep host-key checking enabled for a production cluster.

In Desktop, add the connection under **Infrastructure → Environments**, assign the managed SSH key, and run the connection health check. For API registration, follow the [complete connection tutorial](../../tutorials/register-hpc#through-the-api), which creates and tests the JSON payload before saving the environment. To change a saved connection later, read its current fields before sending a complete `PUT /environment-connections/{connectionId}/` body so unrelated settings remain intact.

## 2. Describe the SLURM resources

The versioned [`examples/slurm/environment.yaml`](https://github.com/UFFeScience/akoflow/blob/v1.0.8/examples/slurm/environment.yaml) provides the catalog portion: runtime, cluster, partition, representative compute node, storage resources, and runtime bindings. Add a real connection like the preceding one before submitting it. The linked v1.0.8 file opts the login node into direct scheduling; set its `schedulable` field to `false` before institutional registration. The current repository example defaults to `false`.

```yaml title="Runtime and resource excerpt from environment.yaml"
runtimes:
  - id: slurm
    driver: slurm
    mode: execution
    role: compute
    configuration:
      partition: cpu
      containerRuntime: apptainer
    capabilities:
      batch: true
      container: true
      sharedStorage: true
      dataStaging: true
      cancellation: true

resources:
  - id: slurm-cluster
    type: cluster
    schedulable: false
  - id: slurm-cpu-partition
    parentResourceId: slurm-cluster
    executionTarget: batch
    type: hpc_partition
    providerId: cpu
    schedulable: true
  - id: slurm-login-node
    parentResourceId: slurm-cluster
    executionTarget: direct
    type: hpc_machine
    providerId: login
    cpuCores: 1
    cpuCapacity: 1
    schedulable: true
```

Bind the runtime to the partition and compute resources. AkôFlow uses the selected partition's `providerId` for `sbatch`, or `configuration.partition` if that ID is absent. A selected batch `hpc_machine` becomes an `sbatch` node target. The login node in the example has a `direct` execution target; leave it unschedulable for workflow planning unless your site explicitly permits lightweight direct work. Interactive console access does not require scheduling workflow activities there.

For a remote SSH connection, AkôFlow submits the batch script through standard input to `sbatch`; it keeps the audit copy in the configured `scriptDirectory` on the daemon host. Ensure that directory exists and is writable by the daemon. The remote login node does not need that local audit path for stdin submission.

## 3. Discover the actual cluster before trusting the catalog

Run discovery after the connection health check. The current SLURM discovery invokes `sinfo` for partition and node facts and collects login-node filesystems, available transfer tools, outbound HTTPS capability, and the installed Apptainer or Singularity version. It can materialize discovered partitions and compute-node records for the connected environment.

Compare discovery with the initial catalog:

1. Confirm the intended partition is available and its name has no trailing `*` in the stored `providerId`.
2. Confirm cores and memory are appropriate for the activities' requests.
3. Confirm the login host is represented separately from compute nodes.
4. Record the required scratch, archive, or project path for the compute-allocation probe in the next step.
5. Review the discovered transfer capabilities before selecting a staging strategy.

Discovery is inventory evidence, not a reservation. A partition shown as available can still queue a job because of account, QoS, dependency, priority, or resource constraints.

## 4. Register compute-visible storage

The example registers a Lustre workspace and an NFS archive separately:

```yaml title="Storage excerpt from environment.yaml"
storages:
  - id: slurm-default-lustre
    name: cluster-scratch
    type: lustre
    endpoint: /scratch/akoflow
    shared: true
    runtimeBindings:
      - runtimeId: slurm
        default: true
        hostPath: /scratch/akoflow
        containerPath: /akoflow/data
  - id: slurm-archive-nfs
    name: experiment-archive
    type: nfs
    endpoint: /archive/akoflow
    shared: true
```

These paths must be valid from the allocated compute node, not merely from the login shell. Submit a small site-approved probe that writes a file to the intended workspace and reads it back from a second allocation. Check ownership, quota, purge policy, and the path exposed inside Apptainer before relying on artifacts or inter-activity data. The catalog entries alone do not enable Desktop file browsing; that also needs a configured browser and [approved roots](./storage).

## 5. Scope, validate, and submit a small real execution

Create an execution scope containing the environment version. The versioned example uses:

```yaml title="examples/slurm/scope.yaml"
id: example-slurm-v1-scope
name: Example Slurm scope
environmentVersionIds:
  - example-slurm-v1
```

In Desktop, choose **Infrastructure → Execution scopes**, select the environment version, then import the workflow. Generate a plan or make a manual plan that targets only the validated partition or compute resources. Review the selected runtime and placement before choosing **Real execution**.

The recommended validation sequence is:

1. connection health with the configured host key and proxy route;
2. SLURM discovery and partition review;
3. a short `sbatch` probe in the intended partition;
4. a compute-node storage write/read probe;
5. a minimal Apptainer job when containers are required;
6. a one-activity AkôFlow run with persisted logs and artifact evidence.

AkôFlow parses `sbatch --parsable` output, uses `sacct` to observe status, and falls back to `squeue` and `scontrol` when accounting is unavailable. A job that disappears from `squeue` is not automatically failed: completed jobs can leave controller memory before accounting catches up. Inspect the activity's persisted log and status-query warning before retrying or cancelling it.

## Queue time, cancellation, and interactive sessions

For SLURM, queue time is the interval after submission before the allocation starts. It is neither transfer time nor container runtime. Inspect the reason shown by `squeue` or `scontrol`: common reasons include `Resources`, `Priority`, `Dependency`, account limits, and `QOSMax*` limits.

Cancelling an active batch activity calls `scancel <job-id>`. Do not delete scheduler-owned files as a substitute for cancellation.

The interactive console uses the same connection and trust route. Selecting a partition starts `srun --partition=<partition> --pty /bin/bash -l`; selecting a compute machine uses `--nodelist=<node>`. Selecting the login node opens a direct SSH shell. Close the console session when finished so AkôFlow can cancel its named interactive allocation.

## Troubleshoot safely

| Symptom | Check and recover |
| --- | --- |
| SSH works from a laptop but fails in AkôFlow | Test from the daemon host. Verify `proxyCommand`, port, managed key file, host-key alias, and known-hosts file on that host. |
| `sbatch` is unavailable | The login node needs SLURM client commands on the PATH. Confirm the selected runtime is bound to the login connection. |
| Job stays pending | Inspect `squeue`/`scontrol` reason and correct the partition, account, QoS, resource request, or dependency rather than inflating the predicted runtime. |
| Job cannot see artifacts or input data | Test the exact storage path from a compute allocation. Review shared-filesystem mounts, container bind behavior, and file permissions. |
| Status looks stale after completion | Check the sentinel/log path and wait for `sacct`; AkôFlow preserves a warning rather than converting missing accounting data into a false failure. |
| Interactive allocation remains after closing the browser view | Close the AkôFlow console session explicitly; it owns the `srun` allocation and cleanup path. |

After a site-approved run, use [Execute and monitor a workflow](../workflows/executions) to inspect its activity status and evidence. Related setup: [SSH service keys](../operations/credentials-and-ssh), [interactive console and commands](../operations/interactive-console), [execution scopes](./execution-scopes), and [storage](./storage).
