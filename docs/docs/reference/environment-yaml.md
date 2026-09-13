---
title: Environment YAML reference
description: Field-level reference for the environment definition accepted by the AkôFlow API.
---

# Environment YAML reference

This reference describes the `EnvironmentDefinition` document accepted by `POST /environments/` and `PUT /environments/{environmentId}/`. JSON and YAML carry the same structure. It is for authors who need a reproducible infrastructure inventory; use the [environment guide](../guides/infrastructure/environments) for the Desktop workflow and the runtime guides for provider-specific setup. “Recommended” fields improve the inventory but are not required by the create handler.

## Before you write a definition

- Use stable, unique IDs. The environment ID is the identity used by `PUT`; send a complete definition when replacing an unused environment. Replacement can fail once a scope, plan, or other record references its inventory. The version ID is the identity referenced by scopes.
- Create the environment before an execution scope. A scope refers to the version ID, and its network topology is a separate document.
- Declare performance values deliberately. The API decodes omitted numeric values as `0`, including `computeSpeedup`, and saves them explicitly. Set a positive speedup and realistic capacity for schedulable resources.
- Keep secrets out of the file. Connection credential references identify saved credentials. Transfer and storage references have provider-specific behavior; see the [AWS/S3 limits](../guides/infrastructure/aws) before using them.

The smallest useful simulation definition is versioned in [`examples/simulation/environment.yaml`](https://github.com/UFFeScience/akoflow/blob/v1.0.8/examples/simulation/environment.yaml). It is a better starting point than an empty document because it includes a runtime, schedulable resources, and their bindings.

## Document shape

```yaml
environment: {}
version: {}
runtimes: []
resources: []
resourceRuntimeBindings: []
resourceRelations: []
storages: []
activityResourceProfiles: []
connections: []
connectionChecks: []
connectorBindings: []
```

`environment` and `version` are required for a persisted definition. The remaining collections may be empty at creation time, but a plan needs at least one schedulable resource, an enabled binding, and a runtime compatible with the selected execution mode.

The create handler returns the submitted document. It may still show omitted nested parent IDs as empty strings. Read `GET /environments/{environmentId}/` to see the IDs actually saved with the environment and version.

## Identity and version fields

| Path | Required | Type | Accepted values / default | Notes |
| --- | --- | --- | --- | --- |
| `environment.id` | Yes | string | unique ID | Primary environment ID. |
| `environment.name` | Yes | string | non-empty in a useful definition | Display name; unique names are not required. |
| `environment.description` | No | string | `""` | Free text. |
| `environment.status` | No | string | `defined` | Documented states are `defined`, `connecting`, `connected`, `discovering`, `ready`, `degraded`, and `unreachable`. The create path does not validate this string; use an observed state rather than treating it as a runtime selector. |
| `environment.createdAt` | No | timestamp | database creation time | Returned by reads; do not author it. |
| `version.id` | Yes | string | unique ID | The immutable ID referenced by scopes. |
| `version.environmentId` | No | string | saved as `environment.id` | The repository uses the enclosing environment ID when it saves the version. |
| `version.version` | Yes | integer | sequence chosen by author | Must be unique for an environment. |
| `version.status` | Recommended | string | `""` if omitted | Documented states are `draft`, `published`, and `retired`; the create path does not validate this string. Use `published` for inventory intended for planning. |
| `version.networkModel` | No | string | `""` if omitted | Optional model label such as `static-links`. Topology links live in a separate network-topology document. |
| `version.interferenceModel` | No | string | `""` if omitted | Record the model assumption when one is known, for example `none`. |
| `version.costModel` | No | string | `""` if omitted | Record the cost interpretation when one is known, for example `per-second`. |
| `version.configurationHash` | Recommended | string | `""` if omitted | Supply a stable label or hash to identify the inventory configuration; the create path does not compute it. |
| `version.createdAt`, `version.publishedAt` | No | timestamp | server-managed / optional | Read-only evidence fields. |

## Runtimes

Each entry in `runtimes` defines how a version can execute or simulate work. `configuration` is a runtime-specific object; see the relevant [SimGrid](../guides/infrastructure/simgrid), [Kubernetes](../guides/infrastructure/kubernetes), or [SLURM/HPC](../guides/infrastructure/hpc-slurm) guide before adding its keys.

| Path | Required | Type | Values / default | Notes |
| --- | --- | --- | --- | --- |
| `runtimes[].id` | Yes | string | unique ID | Referenced by resource and storage bindings. |
| `runtimes[].environmentVersionId` | No | string | saved as `version.id` | The repository uses the enclosing version ID when it saves each runtime. |
| `runtimes[].name` | Yes | string | unique per version | User-facing runtime name. |
| `runtimes[].driver` | Yes | enum | `slurm`, `kubernetes`, `ssh`, `local`, `serverless`, `simgrid`, `cloud` | The database validates this list. |
| `runtimes[].mode` | Yes | enum | `execution`, `simulation` | The database validates this list. A SimGrid runtime uses `simulation`; a remote runtime normally uses `execution`. |
| `runtimes[].role` | No | string | `""` | Informational role, such as `simulation`. |
| `runtimes[].configuration` | No | object | omitted | Driver-specific settings; supply an object when the driver needs one. |
| `runtimes[].capabilities` | Recommended | object | all booleans default to `false` when omitted | Declare only capabilities the runtime actually provides. |

`capabilities` accepts these boolean keys: `batch`, `interactive`, `container`, `serverless`, `gpu`, `mpi`, `sharedStorage`, `dataStaging`, `cancellation`, `logStreaming`, and `simulation`.

## Resources and resource bindings

Resources are the candidates a plan can place activities on. A resource is usable only when it belongs to the selected version, is `schedulable: true`, and has an enabled binding to a compatible runtime.

| Path | Required | Type | Values / default | Notes |
| --- | --- | --- | --- | --- |
| `resources[].id` | Yes | string | unique ID | Referenced by bindings, relations, profiles, assignments, and topology links. |
| `resources[].environmentVersionId` | No | string | saved as `version.id` | The repository uses the enclosing version ID when it saves each resource. |
| `resources[].type` | Recommended | string | `""` if omitted | Use a known resource type below to classify the resource. The create path does not validate this field against that list. |
| `resources[].name` | Yes | string | — | Display name. |
| `resources[].providerId` | Yes | string | unique per version | Provider-facing or modeled identifier. |
| `resources[].executionTarget` | No | enum | `batch` | `batch`, `direct`, or `provisioned`. The create path normalizes an omitted value to `batch`. |
| `resources[].parentResourceId` | No | string | omitted | Parent resource ID, when the hierarchy is meaningful. |
| `resources[].tier`, `region`, `zone`, `architecture` | No | string | `""` | Placement descriptors. |
| `resources[].cpuCores` | Recommended | integer | `0` | Number of modeled cores. Set the actual parallel capacity. |
| `resources[].cpuCapacity` | Recommended | number | `0` | Schedulable CPU capacity. Keep it coherent with `cpuCores` for a one-unit-per-core model. |
| `resources[].memoryBytes`, `storageBytes` | Recommended | integer | `0` | Capacity in bytes. |
| `resources[].computeSpeedup` | Recommended | number | `0` when omitted from API input | Relative compute multiplier. Set a positive value, commonly `1` for the baseline resource. |
| `resources[].pricePerSecond` | Recommended | number | `0` | Cost rate used by planning/simulation. |
| `resources[].bootOverheadSeconds`, `containerOverheadSeconds` | No | number | `0` | Modeled setup delays in seconds. |
| `resources[].schedulable` | Recommended | boolean | `false` when omitted from API input | Set `true` for a placement target. `false` retains inventory visibility without making a placement target. |
| `resources[].metadata` | No | object | omitted | Provider- or experiment-specific metadata. |

Known resource types are: `cluster`, `node_pool`, `kubernetes_machine`, `hpc_partition`, `hpc_machine`, `cloud_vm`, `fog_device`, `local_machine`, `serverless_platform`, `serverless_function`, `batch_queue`, `kubernetes_namespace`, and `slurm_reservation`. A stored type does not by itself make the resource runnable.

```yaml
resourceRuntimeBindings:
  - resourceId: simulated-edge
    runtimeId: simgrid
    enabled: true
    configuration: {}
```

`resourceRuntimeBindings[].resourceId` and `runtimeId` are required and must reference entries in the same definition. Set `enabled: true` for a usable binding; an omitted value decodes as `false` through the API. Omit `configuration` when the binding needs no settings.

`resourceRelations` is optional. When used, each relation needs `sourceResourceId`, `targetResourceId`, and `type`; the repository saves the enclosing `version.id` as its `environmentVersionId`. The allowed relation types are `contains`, `member_of`, and `accessible_via`. A relation cannot point from a resource to itself.

## Connections and transfer connectors

`connections` describes how AkôFlow reaches an environment. It is an environment-level collection, not a version-level collection.

| Path | Required | Type | Values / default | Notes |
| --- | --- | --- | --- | --- |
| `connections[].id`, `name`, `type` | Yes for a usable connection | string | Known types: `ssh`, `kubernetes`, `cloud`, `local`, `agent` | `name` is unique within the environment. The create path does not validate `type` against this list. |
| `connections[].environmentId` | No | string | saved as `environment.id` | The repository uses the enclosing environment ID when it saves each connection. |
| `connections[].endpoint`, `username`, `credentialRef` | No | string | `""` | Reference stored credentials; never put tokens or private keys here. |
| `connections[].configuration` | No | object | omitted | Connection-type-specific settings. |
| `connections[].createdAt` | No | timestamp | server-managed | Read-only evidence field. |

`connectorBindings` declares artifact-transfer capabilities. Its `connector` enum is `rsync`, `scp`, `sftp`, `http`, `s3-compatible`, or `gcs`. The fields `id`, `environmentId`, and `connector` identify the binding; `endpoint`, `credentialRef`, and `configuration` are optional.

The direct S3 transfer connector reads server environment credentials when `credentialRef` is omitted or set to `env`; it does not resolve an arbitrary saved reference. The schema accepts `gcs`, but the current server's direct `gs://` connector returns an unavailable error. A declared binding alone does not make a transfer usable. `health` is observation data; write it from a check, not an assumption.

`connectionChecks` is also observed data. Do not copy a historical `online` result into a new environment file: validate the connection again after import.

## Storage

`storages` records storage that an environment may use; it does not create a bucket, NFS export, PVC, or filesystem. Each storage entry needs an `id`, `name`, and `type`; the repository saves the enclosing `version.id` as its `environmentVersionId`. Database-accepted type values are `local`, `pvc`, `nfs`, `s3`, `lustre`, `gcs`, `s3-compatible`, and `ssh-filesystem`. An accepted type does not prove that the running server can read its bytes; try the intended browse or transfer operation.

| Path | Required | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `storages[].endpoint` | No | string | `""` | Mount, URL, bucket, or filesystem endpoint. |
| `storages[].capacityBytes` | No | integer | `0` | Capacity in bytes; must not be negative. |
| `storages[].shared`, `readOnly` | No | boolean | `false` | Access semantics. |
| `storages[].credentialReference` | No | string | `""` | Recorded reference. The default S3 browser does not resolve it and sends unsigned requests. |
| `storages[].configuration`, `metadata` | No | object | omitted | Storage-provider details. |
| `storages[].configuration.browseRoots[]` | For browsing | array of objects | none | Approved roots, each with a `path`. For local filesystem browsing, include the exact `endpoint` path. |
| `storages[].runtimeBindings[]` | No | array | none | Makes storage available to a runtime. |

A storage runtime binding requires `runtimeId`. Its `containerPath` defaults to `/akoflow/data` when omitted, and `default`, `readOnly`, `hostPath`, and `configuration` are optional. At most one storage can be `default: true` for a given environment version and runtime.

For a self-managed daemon browsing its own filesystem, set `AKOFLOW_LOCAL_STORAGE_ROOT` to an approved directory before starting it. Then register a `local` storage with the same absolute path as `endpoint` and in `configuration.browseRoots`:

```yaml
storages:
  - id: lab-files
    environmentVersionId: lab-v1
    name: Lab files
    type: local
    endpoint: /srv/akoflow-lab
    configuration:
      browseRoots:
        - path: /srv/akoflow-lab
```

Use this excerpt inside a complete environment definition; the directory must exist and be accessible to the daemon. The environment repository persists `configuration`, so a top-level `browseRoots` field alone will not enable browsing. `browseRoots`, `capabilities`, `health`, `indexPolicy`, and `indexStatus` also appear as fields in API responses. A catalog's `healthy` flag means the driver is available; it is not a fresh probe of the storage path or a compute node.

## Activity resource profiles

`activityResourceProfiles` is optional calibration data for scheduling. It links an activity type to a resource and accepts `id`, `activityTypeId`, `resourceId`, `runtimeSeconds`, `runtimeStdDevSeconds`, `cpuUtilization`, `peakMemoryBytes`, `diskReadBytes`, `diskWriteBytes`, `energyJoules`, `source`, `sampleSize`, `modelVersion`, and `metadata`. Durations are seconds; memory and disk values are bytes. The activity type and resource must already exist.

This profile is not a substitute for a workflow activity's `simulation.durationSeconds` or FLOPs profile. For a SimGrid workflow, declare its per-activity compute model in the workflow and use resource profiles only as additional measured calibration data.

## Minimal complete example

```yaml
environment:
  id: lab-sim
  name: Lab simulation
  status: ready
version:
  id: lab-sim-v1
  environmentId: lab-sim
  version: 1
  status: published
  networkModel: static-links
  interferenceModel: none
  costModel: per-second
  configurationHash: lab-sim-v1
runtimes:
  - environmentVersionId: lab-sim-v1
    id: simgrid
    name: SimGrid
    driver: simgrid
    mode: simulation
    capabilities: {simulation: true}
resources:
  - id: edge
    environmentVersionId: lab-sim-v1
    type: fog_device
    name: Edge node
    providerId: edge
    cpuCores: 2
    cpuCapacity: 2
    memoryBytes: 2147483648
    storageBytes: 21474836480
    computeSpeedup: 1
    pricePerSecond: 0
    bootOverheadSeconds: 0
    containerOverheadSeconds: 0
    schedulable: true
resourceRuntimeBindings:
  - resourceId: edge
    runtimeId: simgrid
    enabled: true
```

After creating it, retrieve `GET /environments/lab-sim/` and confirm that the runtime, resource, and binding are present. Then create an [execution scope](../guides/infrastructure/execution-scopes) and its network topology before generating a plan.

## Compatibility and common failures

| Situation | Result and recovery |
| --- | --- |
| A runtime driver or mode is not in the documented enum | SQLite rejects the definition. Use a supported driver/mode pair. |
| A duplicate `environment.id`, version number, provider ID, runtime name, connection name, or binding pair is supplied | The create transaction fails. Choose a new identity or update the existing definition through `PUT`. |
| A resource is not bound to the selected runtime | It is absent from compatible planning resources. Add an enabled resource-runtime binding. |
| A resource has zero capacity or an omitted compute model | Planning may have no feasible placement or a meaningless estimate. Set resource capacities and workflow activity profiles explicitly. |
| Two default storages bind to one runtime | The definition is rejected. Keep one default storage for each runtime/version pair. |
| A scope or plan already uses the environment | Deletion or replacement can fail to preserve existing references. Register the revised inventory as a new environment with new environment and version IDs; the current API has no endpoint to append a version to an existing environment. |

Related reference: [workflow YAML](../internal/workflow-spec), [SimGrid modeling](../guides/infrastructure/simgrid), [storage](../guides/infrastructure/storage), and [execution scopes](../guides/infrastructure/execution-scopes).
