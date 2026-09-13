---

title: Execution scopes and network topology reference
description: Field-level reference for execution scopes and network topologies accepted by the AkôFlow API.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

# Execution scopes and network topology reference

This reference defines the two API documents used to choose environment versions and describe data-transfer links for planning. An `ExecutionScope` selects the versions; a `NetworkTopology` defines links between their resources. For the Desktop sequence and a worked setup, use [Execution scopes and network topologies](/docs/guides/infrastructure/execution-scopes).

The API accepts JSON, `application/yaml`, `application/x-yaml`, and `text/yaml` for both documents.

## Creation order and lifecycle

Create environments and their versions first. Then choose IDs for both the scope and its topology, create the scope, and create the topology.

<img src={useBaseUrl('/img/architecture/scope-topology-lifecycle.svg')} alt="Published environment versions and resources feed an execution scope, which owns a network topology used by planning sessions and plans." />

The current API exposes `POST`, `GET`, and list operations for topologies; it does not expose topology replacement or deletion. For scopes it exposes `POST`, `GET`, list, and `DELETE`; it does not expose replacement. Treat IDs and link values as immutable once they are included in a plan.

An execution scope can be created without `networkTopologyId`. If you want the scope record to name its topology, provide the intended topology ID at scope creation time: there is no scope update endpoint. Planning sessions nevertheless carry their own `networkTopologyId`, so callers should always submit the actual topology ID when creating a planning session.

## Execution scope

`POST /execution-scopes/` creates a scope. `GET /execution-scopes/` lists scopes and `GET /execution-scopes/{scopeId}/` reads one.

```yaml
id: edge-cloud-scope
name: Edge and cloud scope
networkTopologyId: edge-cloud-network-v1
environmentVersionIds:
  - edge-v1
  - cloud-v1
metadata:
  purpose: comparison
```

| Field | Required | Type | Default | Rules and meaning |
| --- | --- | --- | --- | --- |
| `id` | Yes | string | — | Unique scope ID. |
| `name` | Yes | string | — | Display name. |
| `networkTopologyId` | No | string | `""` | Optional topology ID recorded with the scope. No database foreign key validates it at scope creation. |
| `environmentVersionIds` | Yes | array of strings | — | One or more environment-version IDs. Duplicate IDs cause the insert transaction to fail. Each ID must already exist. |
| `metadata` | No | object | omitted | Additional descriptive data. |

The repository rejects a scope with an empty `id`, empty `name`, or no environment versions. A scope cannot be deleted after a schedule plan references it; this preserves the infrastructure context of existing plans.

## Network topology

`POST /network-topologies/` creates a topology. `GET /network-topologies/` lists all topologies and `GET /network-topologies/{topologyId}/` reads one.

```yaml
id: edge-cloud-network-v1
name: Edge to cloud network
version: 1
executionScopeId: edge-cloud-scope
links:
  - id: edge-cloud
    sourceResourceId: edge-node
    targetResourceId: cloud-vm
    bandwidthBitsPerSecond: 100000000
    latencySeconds: 0.05
    pricePerByte: 0.0000000001
    bidirectional: true
    sharingPolicy: shared
    maxConcurrentTransfers: 1
metadata:
  source: measured-lab-link
```

| Field | Required | Type | Default | Rules and meaning |
| --- | --- | --- | --- | --- |
| `id` | Yes | string | — | Unique topology ID. |
| `name` | Yes | string | — | Display name. |
| `version` | Yes | integer | — | Must be greater than zero. |
| `executionScopeId` | Yes | string | — | Existing scope that owns this topology. |
| `links` | No | array | `[]` | Directed link declarations. An empty topology is accepted, but it cannot model cross-resource transfer. |
| `metadata` | No | object | omitted | Additional model or provenance data. |

### Link fields

| Field | Required | Type | Default | Rules and meaning |
| --- | --- | --- | --- | --- |
| `id` | Yes | string | — | Unique within the topology. |
| `topologyId` | No | string | enclosing topology ID | It is populated from the enclosing topology when persisted; omit it in authored input to avoid a stale duplicate. |
| `sourceResourceId` | Yes | string | — | Existing resource ID at the link's source. |
| `targetResourceId` | Yes | string | — | Existing resource ID at the link's destination; it must differ from the source. |
| `bandwidthBitsPerSecond` | Yes | number | — | Strictly positive bandwidth in **bits per second**. |
| `latencySeconds` | No | number | `0` | One-link latency in seconds; cannot be negative. |
| `pricePerByte` | No | number | `0` | Transfer cost per byte; cannot be negative. |
| `bidirectional` | No | boolean | `false` when omitted from API input | Makes the declared link usable in both directions. Set it explicitly when reverse transfers are needed. |
| `sharingPolicy` | No | string | `""` when omitted from API input | Policy passed to the SimGrid platform: `independent` and `fatpipe` become `FATPIPE`; every other value, including `shared` and an omitted API value, becomes `SHARED`. Use `shared` or `independent` explicitly. |
| `maxConcurrentTransfers` | No | integer | `0` | Cannot be negative. It is stored with the topology; treat it as an explicit model limit when your runtime/planner supports it. |
| `metadata` | No | object | omitted | Link provenance or provider-specific context. |

The API validates topology identity, positive version, scope ID, link identity, different endpoints, positive bandwidth, and non-negative latency, price, and concurrency. SQLite also rejects duplicate source/target pairs in one topology.

It does **not** currently validate that the endpoint resources belong to environment versions selected by `executionScopeId`. Keep that invariant in authored files; otherwise the topology can persist but be semantically unsuitable for planning.

## Units, direction, and transfer model

`bandwidthBitsPerSecond` is bits/s, while workflow dependency `sizeBytes` is bytes. For a direct link, the base transfer estimate is:

```text
latencySeconds + sizeBytes / (bandwidthBitsPerSecond / 8)
```

For example, transferring 100,000,000 bytes on a 100,000,000 bit/s link with 0.05 s latency has a base estimate of `8.05 s`.

`bidirectional: true` permits the reverse direction through the same declared link. With `false`, create a second link for reverse traffic when the model needs it. A data dependency only transfers when its producer and consumer are placed on different resources; same-resource dependencies have zero transfer time.

The HEFT baseline finds a matching direct link for its transfer estimate. PRISM additionally uses its network model to account for known competing flows. SimGrid builds the declared links into its platform and maps `shared` (or an unspecified policy) to shared bandwidth. These details explain a model; they do not measure a physical network. Use observed throughput and latency to calibrate a real environment before treating predictions as forecasts.

## API sequence

The checked-in [SimGrid bundle](https://github.com/UFFeScience/akoflow/tree/main/examples/simulation) supplies compatible `scope.yaml` and `topology.yaml` files. Complete [API connection setup](/docs/tutorials/api-access), enter a repository checkout, and create the bundle's environment first. The [SimGrid first-run tutorial](/docs/guides/workflows/first-run) gives the full setup. Then submit these two files in order:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/simulation/scope.yaml \
  "$AKOFLOW_API_URL/execution-scopes/"

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/simulation/topology.yaml \
  "$AKOFLOW_API_URL/network-topologies/"
```

Verify the stored documents before planning:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/execution-scopes/simulation-example-v1-scope/"

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/network-topologies/simulation-network-v1/"
```

## Common failures

| Symptom | Cause and recovery |
| --- | --- |
| Scope creation returns 422 | Supply non-empty `id` and `name`, and at least one existing environment-version ID. |
| Topology creation returns 422 | Check that `version >= 1`, `executionScopeId` is present, every link has distinct endpoints and positive bit/s bandwidth, and no value is negative. |
| A reverse transfer has no modeled route | Set `bidirectional: true` or declare the reverse link explicitly. |
| Transfer time is eight times too small or large | Verify units: links use bits/s; workflow data uses bytes. |
| A resource never appears in a candidate plan | Check that its environment version is in the scope and it is schedulable; then inspect workflow constraints and the algorithm's placement. Runtime bindings are checked when execution starts, not by this planning filter. |
| The scope cannot be deleted | Existing schedule plans reference it. Preserve it for evidence and create a new scope/version for a new experiment. |

Related reference: [Environment YAML](/docs/reference/environment-yaml), [workflow YAML](/docs/internal/workflow-spec), [SimGrid modeling](/docs/guides/infrastructure/simgrid), and [planning](/docs/guides/workflows/planning).
