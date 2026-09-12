---
title: Model a SimGrid environment
description: Configure a reproducible simulated environment with resources, network links, activity profiles, and observable results.
---

# Model a SimGrid environment

This how-to is for users who already know how to import a workflow and want to model the infrastructure it will run on. It uses the checked-in [edge-to-cloud bundle](../../showcase/edge-cloud-simulation) because that bundle contains a resource model, a topology, per-activity simulation profiles, a scope, and a runnable plan.

Use SimGrid when the question is about a modeled platform: placement, parallel capacity, transfers, latency, and simulated cost. Do not use it to validate an SSH, Kubernetes, cloud, or Slurm connection; a SimGrid environment has no remote endpoint to test. For a first end-to-end execution, start with [Run your first simulated workflow](../workflows/first-run).

## Prerequisites

- A running AkôFlow daemon with the SimGrid runner available. The server image includes it; source builds can follow [`examples/simulation/README.md`](https://github.com/UFFeScience/akoflow/blob/v1.0.8/examples/simulation/README.md).
- A local checkout of the repository if you will submit the versioned YAML bundle.
- A workflow with explicit `simulation.durationSeconds` or `simulation.flops` for every activity whose execution time should be modeled.

## 1. Define schedulable resources

Create a simulation environment and bind the `simgrid` runtime to every resource that a plan may use. In Desktop, open **Infrastructure → Environments**, create a simulation environment, add its resources and the SimGrid runtime, then enable a runtime binding for each resource. The API equivalent is the `environment.yaml` in the example bundle.

```yaml title="examples/simulation/environment.yaml"
resources:
  - id: simulated-edge
    cpuCores: 2
    cpuCapacity: 2
    computeSpeedup: 1
    pricePerSecond: 0
    containerOverheadSeconds: 0.1
    schedulable: true
  - id: simulated-cloud
    cpuCores: 8
    cpuCapacity: 8
    computeSpeedup: 4
    pricePerSecond: 0.001
    bootOverheadSeconds: 0.5
    containerOverheadSeconds: 0.2
    schedulable: true
```

The fields have different jobs:

| Field | Effect in a SimGrid model |
| --- | --- |
| `cpuCores` | Becomes the number of cores on the generated SimGrid host. It is also the number of per-core alternatives considered by core-aware planning. Set it to the parallelism you want to model. |
| `cpuCapacity` | Describes schedulable CPU capacity for activity resource requirements. Keep it coherent with `cpuCores` for the simple one-capacity-unit-per-core model used by the bundled examples. |
| `computeSpeedup` | Multiplies the reference compute rate. With a base runtime profile, a speedup of `4` gives one quarter of the reference runtime on that resource. |
| `pricePerSecond` | Charges the resource's active simulated time; it does not create a cloud bill. |
| `bootOverheadSeconds` and `containerOverheadSeconds` | Add modeled setup time. A plan assignment may override these values when it freezes the selected placement. |
| `schedulable` | Makes the resource available to a scope and to planning. Keep non-execution resources out of a placement by setting it to `false`. |

Do not raise `cpuCores` merely to make a predicted makespan smaller. A 50-core resource models 50 simultaneous execution lanes only when the workflow and the resulting plan can use them. The [50-core fan-out Showcase](../../showcase/parallel-50-core) is the worked example for that case.

## 2. Give each activity its own compute profile

The most important input for a meaningful prediction is not the image or command: it is the work associated with each activity. Put the profile on the activity rather than applying one shared default to the workflow.

```yaml title="examples/simulation/workflow.yaml"
activities:
  - name: prepare
    runtime: simgrid
    simulation:
      model: deterministic
      durationSeconds: 4
  - name: analyze
    runtime: simgrid
    simulation:
      model: deterministic
      durationSeconds: 12
  - name: summarize
    runtime: simgrid
    simulation:
      model: deterministic
      durationSeconds: 2
```

`durationSeconds` is the reference duration. When `metadata.baseRuntimeSeconds` is present, the simulator resolves the runtime as `baseRuntimeSeconds / computeSpeedup`; the exported example includes both values consistently. Alternatively, `simulation.flops` can describe the work directly. Do not supply contradictory profiles: use a duration-based profile for a simple calibrated experiment, or FLOPs when you have a measured compute-work model.

After importing, open the workflow definition and inspect every activity. A missing profile makes a plan fall back to information available in the assignment or activity metadata, which is not a substitute for a measured activity profile.

## 3. Model the network before planning

Create a topology for the execution scope. The Desktop scope form creates an empty topology; its current navigation does not expose link creation. Submit `topology.yaml` through the API after creating the scope. The example models one bidirectional edge-to-cloud link:

```yaml title="examples/simulation/topology.yaml"
links:
  - id: edge-cloud
    sourceResourceId: simulated-edge
    targetResourceId: simulated-cloud
    bandwidthBitsPerSecond: 100000000
    latencySeconds: 0.05
    pricePerByte: 0.0000000001
    bidirectional: true
    sharingPolicy: shared
```

Bandwidth is in **bits per second**, while dependency sizes are in **bytes**. For one link, AkôFlow's transfer estimate is:

`latencySeconds + sizeBytes / (bandwidthBitsPerSecond / 8)`

For example, 100,000,000 bytes over 100,000,000 bit/s with 50 ms latency has a base transfer time of `8.05 s`. The SimGrid platform uses the same bandwidth and latency values. A data dependency creates a transfer only when its producer and consumer are assigned to different resources.

`bidirectional: true` makes the link usable in both directions. `sharingPolicy: shared` is emitted as a shared SimGrid link; use `independent` or `fatpipe` only when the modeled link should not share bandwidth. A topology can contain several links: AkôFlow derives an available path between resources and uses the lowest-latency path according to the configured link latencies and bandwidths. A missing route is not a zero-cost transfer; fix the topology or keep the dependent activities on the same resource.

Declare the data itself in the workflow:

```yaml title="examples/simulation/workflow.yaml"
dataDependencies:
  - producerActivity: prepare
    consumerActivity: analyze
    logicalName: dataset.bin
    sizeBytes: 100000000
  - producerActivity: analyze
    consumerActivity: summarize
    logicalName: result.bin
    sizeBytes: 20000000
```

## 4. Freeze the scope and generate or import a plan

Create an execution scope containing the environment version, then attach the topology to that scope. A plan is evaluated against this frozen combination of workflow, scope, resources, and topology.

```yaml title="examples/simulation/scope.yaml"
id: simulation-example-v1-scope
name: Edge cloud simulation scope
environmentVersionIds:
  - simulation-example-v1
```

In Desktop, open the workflow, choose **Generate plan**, select **Simulation**, and choose the scope. Use **Generate plans** to compare algorithms, or **Create manually** to reproduce a known placement. Inspect the candidate Gantt before selecting it: the lane count should reflect the selected resource cores, and cross-resource dependency lines should correspond to the modeled data dependencies.

To submit the checked-in manual plan and run it through the API, complete [API connection setup](../../tutorials/api-access), use a v1.0.8 checkout, and execute the bundle from its root:

```bash
git clone --branch v1.0.8 --depth 1 https://github.com/UFFeScience/akoflow.git akoflow-simgrid
cd akoflow-simgrid
sh examples/simulation/run.sh
```

Use a fresh instance or change all object IDs first. The example script submits the environment, scope, topology, workflow, plan, and execution request in dependency order.

## 5. Verify compute and network evidence

Wait for the run to reach `completed`, then inspect **Plan vs execution**, **Data**, and **Timeline** in Desktop. Through the API, retrieve the run projection:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/execution-runs/simulation-example-run-v1/"
```

The checked-in edge-to-cloud bundle completed on 2026-09-11 with these invariants:

- three settled activities;
- two transfers totaling `120000000` bytes;
- an observed makespan of `21.593 s`.

The run's execution, transfer, queue, and overhead totals are accumulated across activities. They help explain the result, but they are not values to add to the wall-clock makespan. Use the Gantt and the individual activity/transfer records to identify the critical path.

## Common modeling errors

| Symptom | Check and recover |
| --- | --- |
| All activities have nearly the same tiny duration | Re-import the workflow with a distinct `simulation.durationSeconds` or FLOPs profile for every activity. |
| No transfer is reported | Confirm that the data dependency has `sizeBytes > 0`, the selected assignments use different resources, and the scope topology has a route between them. |
| A planned transfer is unrealistically fast | Check units: topology bandwidth is bit/s and data dependency size is bytes. Include link latency. |
| Parallel activities appear in one lane | Check `cpuCores`, activity CPU requirements, and the plan's `coreId` assignments. Then regenerate the plan. |
| A resource is absent from candidate plans | Confirm `schedulable: true`, an enabled `simgrid` runtime binding, enough CPU/memory for the activity, and that its environment version belongs to the scope. |

Related material: [execution scopes](./execution-scopes), [network fan-out](../../showcase/network-fanout), [parallel 50-core fan-out](../../showcase/parallel-50-core), and [the execution evidence guide](../workflows/executions).
