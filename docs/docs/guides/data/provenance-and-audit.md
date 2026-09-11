---
title: Provenance and audit
description: Explore scientific lineage, run safe read-only SQL, and inspect the operational audit trail.
---

# Provenance and audit

AkôFlow exposes two complementary records:

- **Provenance** connects workflows, plans, runs, activities, transfers, and data as scientific evidence.
- **Audit** records operational actions such as discovery, connection use, console access, credentials, and workflow operations.

Use provenance to answer “how was this result produced?” Use audit to answer “what operation happened, when, to which target, and with what outcome?”

## Explore provenance in Desktop

Open **Provenance**. The **Explore** tab loads a server-defined entity catalog. Select an entity, search across its safe projection, apply a field filter, sort a column, and page through the result. The current page can be exported as CSV or JSON.

<img src={require('@site/static/img/interface/provenance/explore-runs.png').default} alt="AkôFlow Desktop Provenance Explore view with the trusted-record catalog, Runs projection, search field, filter control, CSV and JSON exports, and lineage actions for each row." />

*The catalog defines the projections available for exploration. In the **Runs** projection, the row action opens the record details and the lineage action follows its relationship to the selected plan; use the search and export controls only after choosing the record type that answers the question.*

The API exposes the same server-defined catalog and query:

```bash
curl -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/akoflow-api/provenance/entities/"

curl -G -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  --data-urlencode "q=completed" \
  --data-urlencode "filterField=status" \
  --data-urlencode "filterValue=completed" \
  --data-urlencode "page=1" \
  --data-urlencode "pageSize=50" \
  --data-urlencode "sortField=created_at" \
  --data-urlencode "sortOrder=desc" \
  "$AKOFLOW_URL/akoflow-api/provenance/entities/runs/"
```

Entity names and fields are supplied by `/provenance/entities/`; clients should not invent them. Query responses include entity metadata, `items`, `page`, `pageSize`, `total`, and `hasNext`.

## Follow lineage

From an Explore result, choose **Open lineage**, or open the **Lineage** tab and provide an entity and ID. Select `upstream`, `downstream`, or `both`, choose a depth, then inspect nodes and relationships. Any node can become the new root.

```bash
curl -G -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  --data-urlencode "direction=both" \
  --data-urlencode "depth=2" \
  --data-urlencode "maxNodes=300" \
  "$AKOFLOW_URL/akoflow-api/provenance/lineage/runs/$RUN_ID/"
```

The response contains a `root` key, `nodes`, directed `edges`, and `truncated`. Increase depth deliberately: the graph may expand quickly, and the interface caps a request at 300 nodes.

## Query with read-only SQL

The **SQL** tab presents the queryable schema, templates for common investigations, named JSON parameters, result paging, explain, favorites, and local query history.

Only read-only `SELECT` and `WITH` queries are accepted. The Desktop communicates the current service limits as a 10-second execution timeout and 200 rows per page. Fetch the runtime schema instead of assuming table or column names:

```bash
curl -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/akoflow-api/provenance/sql/schema/"

curl -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sql":"SELECT id, status, created_at FROM execution_runs WHERE status = :status ORDER BY created_at DESC",
    "parameters":{"status":"completed"},
    "page":1,
    "pageSize":200
  }' \
  "$AKOFLOW_URL/akoflow-api/provenance/sql/"
```

Send the same payload to `/provenance/sql/explain/` to inspect the query plan without running the ordinary result path. SQL results contain typed `columns`, `items`, pagination information, a `truncated` flag, and elapsed milliseconds.

:::note Local UI state
SQL favorites and recent-query history are stored in the browser profile. They are conveniences, not provenance records, and are not synchronized through the API.
:::

## Inspect the audit trail

Open **Audit** for a chronological table containing time, event type, target, actor, outcome, and summary. Desktop groups the loaded records into discovery/resources, connections, console sessions, workflows, and credentials; **All events** removes this client-side category filter.

The API supports server-side filtering:

```bash
curl -G -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  --data-urlencode "environmentId=$ENVIRONMENT_ID" \
  --data-urlencode "outcome=failed" \
  --data-urlencode "limit=100" \
  "$AKOFLOW_URL/akoflow-api/audit-events/"
```

Available filter parameters are `eventType`, `environmentId`, `resourceId`, `connectionId`, `sessionId`, `executionId`, `outcome`, and `limit`. Outcomes currently include `started`, `succeeded`, and `failed`. The Desktop currently loads the audit list and applies its category tabs locally; use API filters for precise automation.

## Investigation workflow

For a failed or surprising result:

1. Open the execution and identify the run, activity, plan, and produced data IDs.
2. Open **Provenance > Explore**, find the record, and open its lineage.
3. Use **SQL** when the question crosses multiple entities or compares planned and observed values.
4. Open **Audit** to correlate infrastructure, connection, credential, or console operations around the same time.
5. Export the relevant Explore page when evidence must be shared; preserve IDs so another investigator can reproduce the query.

Provenance endpoints return `503 Service Unavailable` when the explorer is not configured, `400 Bad Request` for invalid entity, SQL, or lineage requests, and `500 Internal Server Error` if schema discovery fails.
