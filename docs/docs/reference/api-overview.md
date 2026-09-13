---
title: API overview
description: Authentication, conventions, and current AkôFlow HTTP endpoint groups.
---

# API overview

The AkôFlow Desktop uses the same API available to automation. Paths in the tables below are relative to `AKOFLOW_API_URL`, which includes `/akoflow-api`.

## Connect and authenticate

Follow [API connection setup](/docs/tutorials/api-access) to set the base URL and enter the token without putting it in shell history. The base URL includes `/akoflow-api`. Then check a protected catalog:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environments/"
```

The listen address is configuration-dependent; do not assume the example port in production. When an API token is configured, send `Authorization: Bearer <token>`. `GET` or `HEAD` requests for `/akoflow-api/instance/` and `GET /akoflow-api/preflight/` are public bootstrap operations. All other operations require the token. A daemon without a token is restricted to loopback access.

Browser origins are controlled by the daemon's allowed-origin configuration. Authentication failures return `401`; non-loopback requests to a loopback-only service return `403`.

## Conventions

- JSON requests use `Content-Type: application/json`; most responses are JSON.
- Simple collection responses are normally JSON arrays. Some catalog and query endpoints use `{ "items": [...] }` or a richer pagination envelope; preserve the shape returned by each endpoint.
- IDs in `{braces}` are URL path parameters. Encode user-provided path segments.
- Many collection paths retain a trailing slash; use the route exactly as shown.
- Successful creates generally return `201 Created`; queued work commonly returns `202 Accepted`; deletes commonly return `204 No Content`.
- Errors from kernel-wrapped routes use a JSON `error` message. Validation failures commonly return `400` or `422`; missing records return `404`; unavailable capabilities return `503`.
- An activated archive snapshot is read-only. Requests other than `GET` return `423 Locked`, except instance activation itself.
- Export, download, build output, and console stream routes return non-JSON content.

Check daemon and local build capabilities:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "${AKOFLOW_API_URL%/akoflow-api}/"
curl --fail-with-body "$AKOFLOW_API_URL/preflight/"
```

The authenticated root health check returns `ok`. Public preflight reports server, Docker, and BuildKit availability.

## Instance, search, and operations

| Method | Path | Purpose |
|---|---|---|
| `GET`, `PUT` | `/instance/` | Read or save installation identity/configuration |
| `GET` | `/preflight/` | Report daemon, Docker, and BuildKit readiness |
| `GET` | `/search/` | Global search |
| `GET` | `/audit-events/` | Filter recorded connection, discovery, and console events |
| `GET` | `/instances/` | List archived instances |
| `GET` | `/instances/default/export/` | Export the default instance |
| `POST` | `/instances/import/` | Import an instance archive |
| `POST` | `/instance-activations/{instanceId}/` | Activate an archived instance |
| `POST` | `/factory-reset/` | Clear the active catalog and managed Kubernetes tokens; [retained files need separate cleanup](/docs/guides/operations/instance-management#factory-reset) |
| `GET`, `PUT` | `/user-preferences/{clientId}/` | Read or save client preferences |

## Environments, connections, and credentials

| Method | Path | Purpose |
|---|---|---|
| `GET`, `POST` | `/environments/` | List or create environments |
| `GET`, `PUT`, `DELETE` | `/environments/{environmentId}/` | Read, replace, or delete an environment |
| `POST` | `/connection-tests/` | Test an unpersisted connection definition |
| `PUT` | `/environment-connections/{connectionId}/` | Update a connection |
| `POST` | `/environment-connections/{connectionId}/health/` | Check persisted connection health |
| `POST` | `/environment-connections/{connectionId}/discover/` | Discover environment infrastructure |
| `GET` | `/environment-connections/{connectionId}/history/` | List connection health/discovery history |
| `GET`, `POST` | `/ssh-keys/` | List or generate SSH keys |
| `POST` | `/ssh-keys/import/` | Import an SSH key |
| `POST` | `/kubernetes-tokens/` | Store a Kubernetes token |
| `POST` | `/cloud-credentials/` | Store a cloud credential |
| `POST` | `/cloud-credentials/validate/` | Validate a cloud credential |

## Cloud capacity and machine configuration

| Method | Path | Purpose |
|---|---|---|
| `GET`, `POST` | `/environments/{environmentId}/cloud-capacity-targets/` | List or create capacity targets |
| `DELETE` | `/cloud-capacity-targets/{targetId}/` | Delete a target |
| `GET`, `POST` | `/environments/{environmentId}/cloud-instances/` | List or provision instances |
| `POST` | `/environments/{environmentId}/cloud-provisioning/` | Start a provisioning workflow |
| `GET` | `/cloud-instances/{instanceId}/provisioning-log/` | Read provisioning log |
| `POST` | `/cloud-instances/{instanceId}/{configure,destroy,start,stop,validate}/` | Perform an instance lifecycle action |
| `GET` | `/cloud-operations/` | List cloud operations |
| `GET` | `/cloud-operations/{operationId}/` | Read an operation |
| `GET` | `/cloud-operations/{operationId}/events/` | Read operation events |
| `GET`, `POST` | `/machine-configurations/` | List or create configurations |
| `GET` | `/machine-configurations/{configurationId}/` | Read a configuration |
| `POST` | `/machine-configurations/{configurationId}/versions/` | Add a configuration version |
| `POST` | `/machine-configuration-validations/` | Validate playbook YAML |
| `GET` | `/environments/{environmentId}/cloud-catalog/` | Read cached provider catalog |
| `POST` | `/environments/{environmentId}/cloud-catalog/refresh/` | Refresh provider catalog |

See [Configure cloud capacity](/docs/guides/infrastructure/cloud-capacity) for target and provisioning steps, and [Machine configurations](/docs/guides/infrastructure/machine-configurations) for the optional Ansible setup.

## Resources, topology, and execution scopes

| Method | Path | Purpose |
|---|---|---|
| `GET`, `POST` | `/resources/` | List or create resources |
| `GET` | `/resources/{resourceId}/` | Read a resource |
| `GET` | `/resources/{resourceId}/snapshot/` | Read its observed snapshot |
| `GET`, `POST` | `/network-topologies/` | List or create topologies |
| `GET` | `/network-topologies/{topologyId}/` | Read a topology |
| `GET`, `POST` | `/execution-scopes/` | List or create scopes |
| `GET`, `DELETE` | `/execution-scopes/{scopeId}/` | Read or delete a scope |

## Storage

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/environments/{environmentId}/storages/` | List environment storages |
| `GET` | `/storages/{storageId}/roots/` | List allowed browse roots |
| `GET`, `DELETE` | `/storages/{storageId}/entries/` | Browse or delete an entry (`path` query) |
| `GET` | `/storages/{storageId}/entry/` | Stat an entry (compatibility route) |
| `GET` | `/storages/{storageId}/stat/` | Stat an entry |
| `POST` | `/storages/{storageId}/downloads/` | Prepare a file download |
| `POST` | `/storages/{storageId}/checksum/` | Calculate an entry checksum |
| `POST` | `/storages/{storageId}/copies/` | Queue a cross-storage copy |
| `POST` | `/storages/{storageId}/archives/` | Queue a directory archive |
| `POST` | `/storages/{storageId}/promote-data/` | Promote an entry to scientific data |
| `POST` | `/storages/{storageId}/promote-artifact/` | Promote an executable artifact |
| `GET`, `POST` | `/storages/{storageId}/index-runs/` | List or start indexing |
| `GET` | `/storage-downloads/{downloadId}/` | Read download/archive status |
| `GET` | `/storage-downloads/{downloadId}/content/` | Stream prepared content |

## Workflows and planning

| Method | Path | Purpose |
|---|---|---|
| `GET`, `POST` | `/workflow-definitions/` | List or create workflows |
| `POST` | `/workflow-definitions/import/` | Import workflow JSON or YAML |
| `GET` | `/workflow-definitions/{workflowId}/` | Read a workflow |
| `GET` | `/workflow-definitions/{workflowId}/export/` | Export workflow YAML |
| `POST` | `/workflow-definition-actions/duplicate/{workflowId}/` | Duplicate a workflow |
| `GET`, `POST` | `/schedule-plans/` | List or create plans |
| `POST` | `/schedule-plans/import/` | Import a plan |
| `GET` | `/schedule-plans/{planId}/` | Read a plan |
| `GET` | `/planning-algorithms/` | List scheduler descriptors |
| `GET`, `POST` | `/planning-sessions/` | List or create planning sessions |
| `GET` | `/planning-sessions/{sessionId}/` | Read a session |
| `POST` | `/planning-sessions/{sessionId}/cancel/` | Cancel a session |
| `GET` | `/planning-sessions/{sessionId}/candidates/` | List candidates |
| `GET` | `/planning-sessions/{sessionId}/candidates/{candidateId}/` | Read a candidate |
| `POST` | `/planning-sessions/{sessionId}/candidates/{candidateId}/select/` | Select a candidate and produce a plan |

For a complete workflow request and the required registration order, use [Workflow definitions](/docs/guides/workflows/definitions) or the [SimGrid first-run tutorial](/docs/guides/workflows/first-run).

## Executions

| Method | Path | Purpose |
|---|---|---|
| `GET`, `POST` | `/execution-runs/` | List or create runs |
| `GET` | `/execution-runs/{runId}/` | Read run detail, including related execution data |

List queries accept endpoint-specific pagination and filters. For execution runs, the Desktop uses `page` and `pageSize` and follows `hasNext`.

## Artifacts and builds

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/artifacts/` | List executable artifact versions |
| `POST` | `/artifacts/docker/` | Register a Docker source and SIF build specification |
| `GET` | `/artifact-locations/` | List recorded artifact locations and their availability flags |
| `GET`, `POST` | `/artifact-materializations/` | List or record materializations (`runId` filters list) |
| `POST` | `/build-contexts/` | Upload multipart `context`, or register stored metadata |
| `POST` | `/artifact-builds/` | Create an immutable build specification |
| `GET` | `/artifact-builds/{buildId}/` | Read a build specification |
| `GET` | `/artifacts/{artifactId}/builds/` | List builds for an artifact |
| `GET`, `POST` | `/artifact-builds/{buildId}/runs/` | List or start build runs |
| `GET` | `/build-runs/{runId}/` | Read build-run status and logs |
| `GET` | `/build-runs/{runId}/output/` | Stream SIF output |

For a build request, see [Build an executable](/docs/guides/data/build-executable). To inspect recorded locations and preparation status, see [Artifact locations](/docs/guides/data/artifact-locations). Storage browsing and file registration are covered in [Browse and manage storage](/docs/guides/infrastructure/storage).

## Provenance

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/provenance/sql/schema/` | Read the queryable schema |
| `POST` | `/provenance/sql/` | Execute parameterized read-only SQL |
| `POST` | `/provenance/sql/explain/` | Explain read-only SQL |

See [Trace a result with provenance](/docs/guides/data/provenance) for query examples.

## Console

| Method | Path | Purpose |
|---|---|---|
| `GET`, `POST` | `/console-commands/` | List or execute recorded console commands |
| `GET`, `POST` | `/console-sessions/` | List or open interactive sessions |
| `DELETE` | `/console-sessions/{sessionId}/` | Close a session |
| `GET` | `/console-sessions/{sessionId}/log/` | Export session log |
| `GET` | `/console-sessions/{sessionId}/stream/` | Stream the interactive session |

The stream route is intentionally not wrapped as ordinary JSON. Use the Desktop client or a compatible streaming client rather than treating it as a request/response endpoint.

## Discover payload shapes

This overview intentionally does not duplicate every domain schema. Use these sources in order:

1. Start from a Desktop operation or a task guide to understand the required lifecycle.
2. Fetch server-described catalogs such as planning algorithms, provenance entities, and provenance SQL schema.
3. Use an existing object returned by a list/detail route as the basis for create or replace operations where applicable.
4. Treat identifiers, immutable digests, credential references, and capability flags as opaque unless a specific guide defines them.

Do not send secrets in general resource objects. Credential endpoints store secrets separately, while environment and connector records refer to them.
