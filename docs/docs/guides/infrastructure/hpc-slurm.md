---
title: HPC and SLURM clusters
description: Connect a login node, model partitions and compute nodes, and validate a cluster safely.
---

# HPC and SLURM clusters

An HPC environment is not a single large machine. AkôFlow models the login endpoint, SLURM partition, representative compute capacity, shared storage, and the bindings that connect those records. Jobs are submitted through the scheduler; interactive commands normally target the login node.

The checked-in [`examples/slurm/environment.yaml`](https://github.com/UFFeScience/akoflow/blob/main/examples/slurm/environment.yaml) and [`scope.yaml`](https://github.com/UFFeScience/akoflow/blob/main/examples/slurm/scope.yaml) show the complete object shape.

## Information to collect from the cluster administrator

- login hostname, SSH port, username, and whether a bastion or `ProxyJump` is mandatory;
- SLURM partition names and account/QoS requirements;
- cores and memory available per node, maximum wall time, and allocation limits;
- container runtime (`apptainer` in the example) and permitted image locations;
- shared filesystems and paths visible from both login and compute nodes;
- outbound-network policy from compute nodes;
- host-key fingerprints and the source CIDR allowed for SSH.

## 1. Create the SSH credential and connection

Import a private key or create a service key under **Settings → Credentials**. Then create the execution connection for the login host. If the site requires a proxy, configure the proxy command or bastion on this connection; artifact checks, interactive shells, and runtime operations must all use the same route. A direct SSH test from your laptop is not proof that the daemon can reach the host.

Pin the server host key. Avoid disabling host-key verification. For certificate- or MFA-based sites, confirm that unattended batch submission is allowed before storing any long-lived credential.

## 2. Describe the runtime and resources

Create an environment with a SLURM execution runtime. Record the partition and container runtime and enable only capabilities supported by the site: batch submission, containers, shared storage, data staging, cancellation, and interactive access are separate claims.

Model resources at the level the scheduler needs:

- **Cluster:** an organizational parent, normally not schedulable.
- **Partition:** a schedulable target with SLURM policy and aggregate limits.
- **Login node:** a direct, non-compute endpoint for validation and interactive commands.
- **Compute node profile:** cores, memory, architecture, and performance representative of jobs dispatched to the partition.

Add the `contains` relations and bind the SLURM runtime to the partition. Bind a direct/SSH runtime to the login resource only when interactive commands are allowed there.

## 3. Register shared storage

The paths in the environment must be valid from compute jobs, not only from the login shell. The example separates a Lustre default workspace from an NFS archive. Confirm ownership, quota, purge policy, and whether containers see the same mount path.

Use a small probe job to write a file on the compute node, then read it through the configured post-run path. This catches the common case where `/home` is visible everywhere but a scratch mount differs between login and compute nodes.

## 4. Build an execution scope

Include the environment and only the partitions/resources allowed for the experiment. A scope is a scheduling boundary, not an access-control substitute. Keep unavailable partitions out of the scope so planners do not return assignments that the runtime cannot dispatch.

## 5. Validate before a scientific run

Run these checks in order:

1. SSH connection and pinned host key.
2. `sinfo`/partition discovery through the configured connection.
3. Runtime capability and cancellation probe.
4. Shared-storage write/read probe from a batch job.
5. Minimal container job through Apptainer.
6. One-activity AkôFlow execution with logs and output capture.

Only then submit a large workflow. If a job remains queued, inspect the SLURM reason (`Resources`, `Priority`, `QOSMax*`, `Dependency`, or account limits) rather than treating all waiting time as computation.

## Proxy-aware troubleshooting

When `ssh` reports `Could not resolve hostname` or a connection closes before authentication, verify which process initiated the connection. The daemon, artifact transfer, runtime adapter, and interactive terminal must resolve the same connection record and proxy settings. Test from the daemon host, not just from the Desktop client.

Continue with [Credentials and SSH service keys](../operations/credentials-and-ssh) for key management and [Interactive console and commands](../operations/interactive-console) for login-node access.
