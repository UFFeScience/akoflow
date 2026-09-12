---
title: Environment YAML reference
description: Field-level reference for the environment definition accepted by the AkôFlow API.
---

# Environment YAML reference

This reference describes the `EnvironmentDefinition` document accepted by `POST /environments/` and `PUT /environments/{environmentId}/`. JSON and YAML carry the same structure. It is for authors who need a reproducible infrastructure inventory; use the [environment guide](../guides/infrastructure/environments) for the Desktop workflow and the runtime guides for provider-specific setup.

## Before you write a definition

- Use stable, unique IDs. The environment ID is the identity used by `PUT`; send a complete definition when replacing an existing environment. The version ID is the identity referenced by scopes.
- Create the environment before an execution scope. A scope refers to the version ID, and its network topology is a separate document.
- Declare performance values deliberately. Omitting a numeric value decodes it as `0` (except `computeSpeedup`, which the database defaults to `1`); that is rarely a useful planning model.
- Keep credentials out of the file. `credentialRef` and `credentialReference` name a credential already stored in AkôFlow; they are not the secret itself.

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

## Identity and version fields

| Path | Required | Type | Accepted values / default | Notes |
| --- | --- | --- | --- | --- |
| `environment.id` | Yes | string | unique ID | Primary environment ID. |
| `environment.name` | Yes | string | non-empty in a useful definition | Display name; unique names are not required. |
| `environment.description` | No | string | `""` | Free text. |
| `environment.status` | No | string | `defined` | `defined`, `connecting`, `connected`, `discovering`, `ready`, `degraded`, or `unreachable`. Set it from observed connection state rather than treating it as a runtime selector. |
| `environment.createdAt` | No | timestamp | database creation time | Returned by reads; do not author it. |
| `version.id` | Yes | string | unique ID | The immutable ID referenced by scopes. |
| `version.environmentId` | Yes | string | `environment.id` | Keep it equal to the enclosing environment ID. The create path persists the enclosing ID. |
| `version.version` | Yes | integer | sequence chosen by author | Must be unique for an environment. |
| `version.status` | Yes | string | `draft`, `published`, `retired` | Use `published` for an inventory intended for a scope. |
| `version.networkModel` | Yes | string | author-defined label | A descriptive model label such as `static-links`; topology links themselves live in the execution-scope document. |
| `version.interferenceModel` | Yes | string | author-defined label | Record the model assumption, for example `none`. |
| `version.costModel` | Yes | string | author-defined label | Record the cost interpretation, for example `per-second`. |
| `version.configurationHash` | Yes | string | author-provided stable hash/label | Used to identify the inventory configuration. |
| `version.createdAt`, `version.publishedAt` | No | timestamp | server-managed / optional | Read-only evidence fields. |

## Runtimes

Each entry in `runtimes` defines how a version can execute or simulate work. `configuration` is a runtime-specific object; see the relevant [SimGrid](../guides/infrastructure/simgrid), [Kubernetes](../guides/infrastructure/kubernetes), or [SLURM/HPC](../guides/infrastructure/hpc-slurm) guide before adding its keys.

| Path | Required | Type | Values / default | Notes |
| --- | --- | --- | --- | --- |
| `runtimes[].id` | Yes | string | unique ID | Referenced by resource and storage bindings. |
| `runtimes[].environmentVersionId` | Yes | string | `version.id` | Keep equal to the enclosing version ID; create persists the enclosing version. |
| `runtimes[].name` | Yes | string | unique per version | User-facing runtime name. |
| `runtimes[].driver` | Yes | enum | `slurm`, `kubernetes`, `ssh`, `local`, `serverless`, `simgrid`, `cloud` | The database validates this list. |
| `runtimes[].mode` | Yes | enum | `execution`, `simulation` | The database validates this list. A SimGrid runtime uses `simulation`; a remote runtime normally uses `execution`. |
| `runtimes[].role` | No | string | `""` | Informational role, such as `simulation`. |
| `runtimes[].configuration` | No | object | `{}` | Driver-specific settings. |
| `runtimes[].capabilities` | Recommended | object | all booleans default to `false` when omitted | Declare only capabilities the runtime actually provides. |

`capabilities` accepts these boolean keys: `batch`, `interactive`, `container`, `serverless`, `gpu`, `mpi`, `sharedStorage`, `dataStaging`, `cancellation`, `logStreaming`, and `simulation`.

## Resources and resource bindings

Resources are the candidates a plan can place activities on. A resource is usable only when it belongs to the selected version, is `schedulable: true`, and has an enabled binding to a compatible runtime.

| Path | Required | Type | Values / default | Notes |
| --- | --- | --- | --- | --- |
| `resources[].id` | Yes | string | unique ID | Referenced by bindings, relations, profiles, assignments, and topology links. |
| `resources[].environmentVersionId` | Yes | string | `version.id` | Keep equal to the enclosing version ID; create persists the enclosing version. |
| `resources[].type` | Yes | enum | See resource types below | Classifies the resource. |
| `resources[].name` | Yes | string | — | Display name. |
| `resources[].providerId` | Yes | string | unique per version | Provider-facing or modeled identifier. |
| `resources[].executionTarget` | No | enum | `batch` | `batch`, `direct`, or `provisioned`. The create path normalizes an omitted value to `batch`. |
| `resources[].parentResourceId` | No | string | omitted | Parent resource ID, when the hierarchy is meaningful. |
| `resources[].tier`, `region`, `zone`, `architecture` | No | string | `""` | Placement descriptors. |
| `resources[].cpuCores` | Recommended | integer | `0` | Number of modeled cores. Set the actual parallel capacity. |
| `resources[].cpuCapacity` | Recommended | number | `0` | Schedulable CPU capacity. Keep it coherent with `cpuCores` for a one-unit-per-core model. |
| `resources[].memoryBytes`, `storageBytes` | Recommended | integer | `0` | Capacity in bytes. |
| `resources[].computeSpeedup` | Recommended | number | `1` | Relative compute multiplier. The database defaults an omitted value to `1`; set it explicitly in portable YAML. |
| `resources[].pricePerSecond` | Recommended | number | `0` | Cost rate used by planning/simulation. |
| `resources[].bootOverheadSeconds`, `containerOverheadSeconds` | No | number | `0` | Modeled setup delays in seconds. |
| `resources[].schedulable` | Recommended | boolean | database default `true` | Set explicitly. `false` retains inventory visibility without making a placement target. |
| `resources[].metadata` | No | object | `{}` | Provider- or experiment-specific metadata. |

Accepted resource types are: `cluster`, `node_pool`, `kubernetes_machine`, `hpc_partition`, `hpc_machine`, `cloud_vm`, `fog_device`, `local_machine`, `serverless_platform`, `serverless_function`, `batch_queue`, `kubernetes_namespace`, and `slurm_reservation`.

```yaml
resourceRuntimeBindings:
  - resourceId: simulated-edge
    runtimeId: simgrid
    enabled: true
    configuration: {}
```

`resourceRuntimeBindings[].resourceId` and `runtimeId` are required and must reference entries in the same definition. `enabled` defaults to `true` in the database, but set it explicitly. `configuration` is optional and defaults to `{}`.

`resourceRelations` is optional. When used, each relation needs `sourceResourceId`, `targetResourceId`, and `type`; `environmentVersionId` should equal `version.id`. The allowed relation types are `contains`, `member_of`, and `accessible_via`. A relation cannot point from a resource to itself.

## Connections and transfer connectors

`connections` describes how AkôFlow reaches an environment. It is an environment-level collection, not a version-level collection.

| Path | Required | Type | Values / default | Notes |
| --- | --- | --- | --- | --- |
| `connections[].id`, `name`, `type` | Yes | string / enum | `ssh`, `kubernetes`, `cloud`, `local`, `agent` | `name` is unique within the environment. |
| `connections[].environmentId` | Yes | string | `environment.id` | Keep it equal to the enclosing environment; create persists the enclosing ID. |
| `connections[].endpoint`, `username`, `credentialRef` | No | string | `""` | Reference stored credentials; never put tokens or private keys here. |
| `connections[].configuration` | No | object | `{}` | Connection-type-specific settings. |
| `connections[].createdAt` | No | timestamp | server-managed | Read-only evidence field. |

`connectorBindings` declares artifact-transfer capabilities. Its `connector` enum is `rsync`, `scp`, `sftp`, `http`, `s3-compatible`, or `gcs`. The fields `id`, `environmentId`, and `connector` identify the binding; `endpoint`, `credentialRef`, and `configuration` are optional. `health` is observation data and should be written by a check rather than authored as an assumption.

`connectionChecks` is also observed data. Do not copy a historical `online` result into a new environment file: validate the connection again after import.

## Storage

`storages` records accessible storage; it does not create a bucket, NFS export, PVC, or filesystem. Each storage entry requires `id`, `environmentVersionId`, `name`, and `type`. Supported types are `local`, `pvc`, `nfs`, `s3`, `lustre`, `gcs`, `s3-compatible`, and `ssh-filesystem`.

| Path | Required | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `storages[].endpoint` | No | string | `""` | Mount, URL, bucket, or filesystem endpoint. |
| `storages[].capacityBytes` | No | integer | `0` | Capacity in bytes; must not be negative. |
| `storages[].shared`, `readOnly` | No | boolean | `false` | Access semantics. |
| `storages[].credentialReference` | No | string | `""` | Stored credential reference. |
| `storages[].configuration`, `metadata` | No | object | `{}` | Storage-provider details. |
| `storages[].runtimeBindings[]` | No | array | none | Makes storage available to a runtime. |

A storage runtime binding requires `runtimeId`. Its `containerPath` defaults to `/akoflow/data` when omitted, and `default`, `readOnly`, `hostPath`, and `configuration` are optional. At most one storage can be `default: true` for a given environment version and runtime.

`browseRoots`, `capabilities`, `health`, `indexPolicy`, and `indexStatus` are inventory/evidence fields returned by the API. Let discovery and indexing populate them; do not rely on an authored health status as proof that storage is reachable.

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
| A scope or plan already uses the environment | Deletion is blocked to preserve reproducibility. Create a new version instead of mutating historical infrastructure. |

Related reference: [workflow YAML](../internal/workflow-spec), [SimGrid modeling](../guides/infrastructure/simgrid), [storage](../guides/infrastructure/storage), and [execution scopes](../guides/infrastructure/execution-scopes).
