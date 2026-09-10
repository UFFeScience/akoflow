---
title: Execution scopes and network topologies
---

An execution scope is a reusable set of environment versions available to planning. A network topology describes transfer links between resources. The scope stores a `networkTopologyId`; the topology stores its `executionScopeId`. Use stable IDs and create the scope before the topology when building them through the current API.

## Create a scope

### Using AkôFlow Desktop

1. Open **Infrastructure → Execution scopes**.
2. Select **Create scope**.
3. Enter an ID and name.
4. Select one or more published environment versions.
5. Leave **Create the initial network topology** selected if you want Desktop to create an empty topology with the scope.
6. Save, then open the scope detail to review its members.

<!-- screenshot: Execution scopes catalog and Create scope action annotated -->

<!-- screenshot: Scope form with environment-version selection annotated -->

The catalog shows the version IDs in each scope. A scope is not a copy of its environments and does not create connections or resources.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_URL/execution-scopes/" \
  -d '{
    "id":"hybrid-research",
    "name":"Hybrid research",
    "networkTopologyId":"hybrid-network-v1",
    "environmentVersionIds":["hpc-v1","cloud-v1"]
  }'
```

List or inspect scopes with:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_TOKEN" "$AKOFLOW_URL/execution-scopes/"
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_TOKEN" "$AKOFLOW_URL/execution-scopes/hybrid-research/"
```

## Add a network topology

### Using AkôFlow Desktop

The current Desktop scope flow can create an empty initial topology, but it does not provide a link editor. Create a topology containing links through the API. Each link identifies source and target **resource IDs**, bandwidth in bits per second, latency in seconds, transfer price per byte, and whether traffic is bidirectional.

<!-- screenshot: Create scope form with the initial-network-topology checkbox annotated -->

Topology values affect transfer estimates. They do not test the physical network and are not produced by a connection health check.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_URL/network-topologies/" \
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
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/network-topologies/hybrid-network-v1/"
```

Use resource IDs that belong to environment versions in the scope. The API validates persistence constraints but does not measure whether the bandwidth and latency values match the real infrastructure.
