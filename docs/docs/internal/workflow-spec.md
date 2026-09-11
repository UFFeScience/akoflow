---
id: workflow-spec
title: Portable workflow specification
sidebar_label: Workflow specification
description: Current YAML and JSON authoring contract accepted by the AkôFlow workflow API.
---

AkôFlow accepts a compact, portable workflow document and normalizes it into the versioned domain model used by planning and execution. This page documents the **authoring contract**, not the larger persisted API response. It is a reference: use [Workflow definitions](../guides/workflows/definitions) for the Desktop/API procedure and the SimGrid guide when modeling a simulation experiment.

Submit YAML or JSON to `POST /akoflow-api/workflow-definitions/` or `/workflow-definitions/import/`. Exporting a workflow produces this portable format without generated IDs or resolved runtime state.

## Complete example

```yaml
name: astronomy-fanout
spec:
  namespace: research
  image: python:3.12
  activities:
    - name: prepare
      cpuLimit: "0.5"
      memoryLimit: 256Mi
      run: python /app/prepare.py
      simulation:
        model: fixed-duration
        durationSeconds: 4

    - name: analyze-a
      cpuLimit: "1"
      memoryLimit: 1Gi
      dependsOn: [prepare]
      command:
        executable:
          source:
            type: oci
            reference: ghcr.io/example/analyzer:1.0
          delivery:
            strategy: auto
        entrypoint: python
        arguments: [/app/analyze.py, --partition, a]

    - name: combine
      cpuLimit: 250m
      memoryLimit: 128Mi
      dependsOn: [analyze-a]
      run: python /app/combine.py

  dataDependencies:
    - producerActivity: prepare
      consumerActivity: analyze-a
      logicalName: prepared-catalog
      sizeBytes: 104857600
```

:::important Real and simulated capabilities
In the current portable importer, an activity with `simulation` is normalized as simulation-capable; an activity without it is normalized as real-capable. Do not assume that adding simulation fields creates one activity that runs in both modes.
:::

## Top-level fields

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `name` | string | yes | Display name and source for the stable workflow identifier |
| `spec.namespace` | string | yes | Logical namespace for the definition |
| `spec.image` | string | no | Default OCI image for real activities that do not declare an executable |
| `spec.activities` | array | yes | Ordered activity definitions |
| `spec.dataDependencies` | array | no | Explicit data edges and their logical sizes |
| `spec.storageClassName`, `storageSize`, `storagePolicy`, `mountPath` | object/string | no | Accepted legacy portable fields. The current portable importer does not persist or apply them; configure storage through the environment and runtime instead. |

The importer derives a lowercase, hyphenated workflow ID from `name`, creates version `1`, and generates activity IDs inside that workflow namespace. Names that normalize to the same identifier can fail persistence; use distinct lowercase-hyphenated names when portability matters.

## Activity fields

| Field | Type | Default | Meaning |
|---|---|---|---|
| `name` | string | required | Unique activity name within the document |
| `run` | string | — | Shell shorthand, normalized to `sh -c <run>` |
| `command` | object | — | Structured executable and command declaration |
| `image` | string | `spec.image` | Per-activity OCI shorthand |
| `runtime` | string | — | Runtime-selection hint retained as metadata |
| `cpuLimit` | string | `0.1` | CPU demand; accepts decimal cores or millicores such as `250m` |
| `memoryLimit` | string | `16Mi` | Bytes or a `Ki`, `Mi`, or `Gi` value |
| `dependsOn` | string[] | `[]` | Names of predecessor activities |
| `resourceSelector` | string | — | Resource-selection hint retained as metadata |
| `keepDisk` | boolean | `false` | Disk-retention hint |
| `mountPath` | string | — | Per-activity mount hint |
| `simulation` | object | — | Simulation model. Its presence makes the portable activity **simulation-only**. |

For a real activity, AkôFlow requires an effective executable and an entrypoint. `run` supplies an `sh -c` entrypoint automatically, while `image` or `spec.image` supplies an OCI executable automatically. A portable activity cannot currently be both real-capable and simulation-capable: adding `simulation` switches its capability to `simulation` and the importer no longer requires a real command.

### Structured command

```yaml
command:
  executable:
    source:
      type: oci
      reference: python:3.12
    delivery:
      strategy: auto
  entrypoint: python
  arguments: [/app/task.py, --output, result.json]
  environment:
    LOG_LEVEL: info
  workingDirectory: /workspace
```

`command` supports `entrypoint`, `arguments`, `environment`, `workingDirectory`, and `executable`. The server adds resolved executable state later; authored documents must not include `resolvedExecutable`.

### Executable source

| `source.type` | Required locator |
|---|---|
| `catalog` | `artifactRef.id`, with optional version |
| `oci` | `reference` |
| `local-container-image` | `reference` |
| `build` | `artifactBuildRef` |
| `local-file` | `path` |
| `remote-file` | `path` and either `environmentRef` or `resourceRef` |
| `object-storage` | `uri` |
| `http` | `uri` |

A source may also carry `expectedDigest`, `format`, or `credentialRef` when appropriate.

Delivery strategies are `auto`, `managed`, `use-in-place`, `destination-pull`, `gateway-transfer`, `build-and-transfer`, and `prefer-in-place`. Target executable formats currently include `oci` and `sif`.

### Simulation duration and FLOPs

```yaml
simulation:
  model: fixed-duration
  durationSeconds: 12.5
  flops: 5000000000
  parameters:
    dataset: small
```

The simulation object accepts `model`, `durationSeconds`, `flops`, and arbitrary `parameters`. The fields are not interchangeable in the current implementation:

| Input | Planning behavior | SimGrid runner behavior | Authoring guidance |
| --- | --- | --- | --- |
| `durationSeconds > 0` | Used as the activity's reference duration, then divided by the resource's `computeSpeedup` unless a resource profile overrides it. | Converted to FLOPs using the selected/frozen runtime when no FLOPs value is supplied. | Set this for every simulated activity when you want a meaningful schedule prediction. |
| `flops > 0` | Not used by the current HEFT/PRISM duration estimator; without `durationSeconds` or a positive resource profile the planner starts from its 1 s fallback. | Used directly as the task's work. | Use only with a calibrated resource compute rate, and also set `durationSeconds` for present planning fidelity. |
| `parameters.flops > 0` | Not used by the duration estimator. | Used as a runner fallback after `simulation.flops`. | Prefer the explicit `flops` field. |
| neither duration nor FLOPs | Planner uses its 1 s reference fallback when no positive resource profile exists. | A manually created plan may supply a frozen predicted duration; otherwise the runner has no useful compute work to model. | Avoid this for simulation workflows. |

When both a positive resource profile and `durationSeconds` exist, the resource profile wins for planning. A per-activity duration must therefore remain in the exported workflow even when an environment also has calibration profiles. Do not substitute one shared minimum duration for a heterogeneous workflow: the planner cannot distinguish heavy and light activities if the source durations are absent.

## Dependencies

`dependsOn` creates control edges. Every referenced name must identify another activity in the same document. The importer rejects unknown names; the database rejects an activity depending on itself. A cycle can persist through the importer but planning rejects it with `workflow contains a cycle`, so validate the DAG before starting a planning session.

Use `dataDependencies` when a **control dependency already exists** and planning also needs the logical data size between the same producer and consumer:

```yaml
dataDependencies:
  - producerActivity: preprocess
    consumerActivity: train
    logicalName: normalized-dataset
    sizeBytes: 2147483648
```

`logicalName` must be non-empty and `sizeBytes` must be positive. Producer and consumer names must exist in `spec.activities`; they must differ; and the tuple `(producerActivity, consumerActivity, logicalName)` must be unique.

`dataDependencies` does **not** create an execution edge. The planner and the SimGrid runner attach bytes to existing `dependsOn` control edges. If `train` consumes `preprocess` output, declare both fields:

```yaml
activities:
  - name: preprocess
    simulation: {model: deterministic, durationSeconds: 12}
  - name: train
    dependsOn: [preprocess]
    simulation: {model: deterministic, durationSeconds: 40}
dataDependencies:
  - producerActivity: preprocess
    consumerActivity: train
    logicalName: normalized-dataset
    sizeBytes: 2147483648
```

If the two activities are placed on different resources, this edge can become a network transfer. If they share a resource, its modeled transfer time is zero. A data dependency without the matching `dependsOn` entry is stored but does not make the consumer wait and does not create a transfer in the current planning/SimGrid execution path.

## Parsing defaults and compatibility

| Input | Normalized result |
| --- | --- |
| Empty `cpuLimit` | `0.1` CPU core. `250m` becomes `0.25`; a decimal such as `0.5` is also accepted. |
| Empty `memoryLimit` | `16Mi` (`16777216` bytes). `Ki`, `Mi`, `Gi`, or an integer byte value are accepted. |
| `run` | `command.entrypoint: sh` and `command.arguments: ["-c", run]`. |
| Activity `image` or `spec.image` on a real activity without `command.executable` | OCI executable with `delivery.strategy: auto`. |
| Missing `simulation` | `real` capability; a command and executable must resolve. |
| Present `simulation` | `simulation` capability; a real command is optional and will not make the activity real-capable. |
| `runtime`, `resourceSelector`, `keepDisk`, `mountPath` | Stored as activity metadata hints. They do not themselves allocate a resource or select an execution mode. |
| `resolvedExecutable` in input | Not part of the authoring contract. It is server-resolved state and is removed on export. |

## Normalized model

The API response is richer than the submitted document. It contains the workflow and version IDs, normalized activities, capabilities, resources in bytes/cores, structured dependencies, policies, priorities, and runtime resolution state. Plans refer to `version.id`, not to the mutable authoring file.

See [Workflow definitions](../guides/workflows/definitions) for Desktop and API procedures, [SimGrid modeling](../guides/infrastructure/simgrid) for a calibrated simulated workflow, and [execution scopes and topologies](../reference/execution-scopes-and-topologies) for the network model used by a plan.
