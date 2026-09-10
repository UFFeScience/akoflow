---
id: workflow-spec
title: Portable workflow specification
sidebar_label: Workflow specification
description: Current YAML and JSON authoring contract accepted by the AkôFlow workflow API.
---

AkôFlow accepts a compact, portable workflow document and normalizes it into the versioned domain model used by planning and execution. This page documents the **authoring contract**, not the larger persisted API response.

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
| `spec.storageClassName` | string | no | Legacy portable storage hint retained by the request contract |
| `spec.storageSize` | string | no | Legacy portable storage-size hint |
| `spec.storagePolicy.type` | string | no | Legacy portable storage-policy hint |
| `spec.mountPath` | string | no | Default portable mount hint |

The importer derives a lowercase, hyphenated ID from `name`, creates version `1`, and generates stable activity IDs inside that workflow namespace.

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
| `simulation` | object | — | Simulation model for a simulated activity |

For a real activity, AkôFlow requires an effective executable and an entrypoint. `run` supplies the entrypoint automatically, while `image` or `spec.image` supplies an OCI executable automatically.

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

### Simulation

```yaml
simulation:
  model: fixed-duration
  durationSeconds: 12.5
  flops: 5000000000
  parameters:
    dataset: small
```

The simulation object accepts `model`, `durationSeconds`, `flops`, and arbitrary `parameters`. The configured simulation runtime determines how the model is interpreted.

## Dependencies

`dependsOn` creates control edges. Every referenced name must identify another activity in the same document.

Use `dataDependencies` when planning also needs the logical data size between two activities:

```yaml
dataDependencies:
  - producerActivity: preprocess
    consumerActivity: train
    logicalName: normalized-dataset
    sizeBytes: 2147483648
```

`logicalName` must be non-empty and `sizeBytes` must be positive. Producer and consumer names must exist in `spec.activities`.

## Normalized model

The API response is richer than the submitted document. It contains the workflow and version IDs, normalized activities, capabilities, resources in bytes/cores, structured dependencies, policies, priorities, and runtime resolution state. Plans refer to `version.id`, not to the mutable authoring file.

See [Workflow definitions](../guides/workflows/definitions) for Desktop and API procedures.
