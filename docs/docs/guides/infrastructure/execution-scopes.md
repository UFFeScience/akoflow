---
title: Define execution scopes and network links
---

An execution scope is a reusable set of environment versions available to planning. A network topology describes transfer links between resources. The scope stores a `networkTopologyId`; the topology stores its `executionScopeId`. Use stable IDs and create the scope before the topology when building them through the current API.

For the API commands on this page, complete [API connection setup](../../tutorials/api-access) first.

## Create a scope

### Using AkôFlow Desktop

1. Open **Infrastructure → Execution scopes**.
2. Select **Create scope**.
3. Enter an ID and name.
4. Select one or more published environment versions.
5. Leave **Create the initial network topology** selected if you want Desktop to create an empty topology with the scope.
6. Save, then open the scope detail to review its members.

<img src={require('@site/static/img/interface/infrastructure/execution-scopes.png').default} alt="AkôFlow Desktop Execution scopes catalog showing scope names, stable IDs, member environment versions and the Create scope action." />

*The catalog is a compact inventory: every row shows the scope name, its stable ID, the environment version selected for it and the number of member environments. Opening a row exposes the scope detail; it does not create a new connection or resource.*

A scope is not a copy of its environments and does not create connections or resources.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_API_URL/execution-scopes/" \
  -d '{
    "id":"hybrid-research",
    "name":"Hybrid research",
    "networkTopologyId":"hybrid-network-v1",
    "environmentVersionIds":["hpc-v1","cloud-v1"]
  }'
```

List or inspect scopes with:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$AKOFLOW_API_URL/execution-scopes/"
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$AKOFLOW_API_URL/execution-scopes/hybrid-research/"
```

## Add a network topology

### Using AkôFlow Desktop

The current Desktop scope flow can create an empty initial topology, but it does not provide a link editor. Create a topology containing links through the API. Each link identifies source and target **resource IDs**, bandwidth in bits per second, latency in seconds, transfer price per byte, and whether traffic is bidirectional.

Topology values affect transfer estimates. They do not test the physical network and are not produced by a connection health check.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_API_URL/network-topologies/" \
  -d '{
    "id":"hybrid-network-v1",
    "name":"HPC to cloud",
    "version":1,
    "executionScopeId":"hybrid-research",
    "links":[{
      "id":"hpc-cloud",
      "topologyId":"hybrid-network-v1",
      "sourceResourceId":"hpc-cluster",
      "targetResourceId":"cloud-capacity-small",
      "bandwidthBitsPerSecond":1000000000,
      "latencySeconds":0.03,
      "pricePerByte":0,
      "bidirectional":true
    }]
  }'
```

Retrieve the stored model before using it for planning:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/network-topologies/hybrid-network-v1/"
```

Use resource IDs that belong to environment versions in the scope. The API validates persistence constraints but does not measure whether the bandwidth and latency values match the real infrastructure.
