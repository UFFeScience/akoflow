---
id: concepts
title: Core concepts
sidebar_label: Core concepts
---

AkôFlow separates infrastructure description, workflow intent, scheduling decisions, and execution observations. This makes a run reproducible and lets the same workflow be planned against different infrastructure.

## Object chain

```text
Environment -> published version -> Execution scope
                                      |
Workflow -> immutable version --------+-> Planning session
                                              |
                                      candidate / Schedule plan
                                              |
                                         Execution run
                                              |
                           tasks, transfers, artifacts, provenance
```

## Environment

An environment names an infrastructure boundary such as a local machine, Kubernetes cluster, SSH host, Slurm cluster, simulation model, or cloud configuration. States are `defined`, `connecting`, `connected`, `discovering`, `ready`, `degraded`, and `unreachable`.

Connections contain access metadata and a credential reference. Connection checks report `online` or `offline`, latency, and diagnostics. Discovery creates inventory; it does not execute workloads.

### Published version

Planning uses an immutable environment version. Versions are `draft`, `published`, or `retired`. A definition can contain runtimes, resources and hierarchy, runtime bindings, storage, relations, profiles, connections, connector bindings, and capability observations.

## Resource, runtime, and scope

A resource is schedulable capacity: a local machine, Kubernetes machine/node pool, HPC partition/machine, cloud VM, serverless target, queue, namespace, or reservation. `executionTarget` distinguishes `batch`, `direct`, and `provisioned` capacity.

A runtime says **how** to launch work. Drivers include `local`, `kubernetes`, `ssh`, `slurm`, `serverless`, `simgrid`, and `cloud`; runtime mode is `execution` or `simulation`. A binding determines which runtime may use a resource.

An execution scope selects published environment versions and a network topology. Directed links model bandwidth, latency, byte price, sharing, and concurrency, allowing planners to account for data movement.

## Workflow

A definition owns identity and namespace; an immutable version contains activities plus control and data dependencies. Activities declare:

- kind: `task`, `service`, or `interactive`;
- capabilities: `real`, `simulation`, and/or `interactive`;
- executable/image, entrypoint, arguments, environment, and working directory;
- CPU, memory, storage, and optional GPU requirements;
- optional simulation and service specifications;
- timeout and retry policy.

Control dependencies order activities. Data dependencies also identify producer, consumer, logical name, and size so movement can be planned and observed.

## Executables and outputs

An **executable artifact** is immutable runnable input, with digest-addressed variants, format, architecture, locations, builds, and materializations. An **artifact manifest** is observed output from an activity and feeds the data catalog and provenance. They are not interchangeable.

## Planning session and plan

A session snapshots workflow, scope, inventory, topology, profiles, algorithms, deadline, budget, and optional interference data. States are `queued`, `running`, `completed`, `failed`, and `cancelled`.

Candidates expose feasibility, predicted makespan, and cost. A plan records assignments, predicted ready/start/finish/runtime/transfer values, resource order, and optional lifecycle actions. Its source may be `plugin`, `manual`, or `imported`. Planning never starts execution automatically.

## Execution run

A run binds a plan to `real`, `simulation`, or `interactive` mode. Run states are `pending`, `running`, `completed`, and `failed`; activity handles are `starting`, `running`, `completed`, `failed`, and `stopped`. These replace the retired public `Ready`/`In Execution`/`Finished` vocabulary.

The supervisor starts an activity only after predecessors complete and its preparation gate commits. It stores observed timing, queue delay, logs, exit code, output manifest, and transfers alongside predicted plan values.

## Data preparation

Materializations progress through `planned`, `reconciling`, `transferring`, `verifying`, `committed`, or `failed`. Transfers are `planned`, `running`, `completed`, or `failed`. Routes retain strategy, endpoints, network domain, fallback, reason, logical bytes, and actual network bytes. An existing verified copy may satisfy a dependency without transfer.

## Cloud capacity

A capacity target is schedulable intent; a provisioned instance is observed infrastructure. Plans may add create/start actions. Execution resolves the instance to a runtime allocation, waits until usable, and later stops or destroys it according to policy.

## Provenance, audit, and reproducibility

- **Provenance** links scientific entities and data lineage.
- **Audit** records operational and security-relevant actions.
- **Planning snapshots** preserve what an algorithm saw even after discovery changes live inventory.

Together they explain what was intended, why placement was chosen, what ran, which bytes moved, what was produced, and who changed state.
