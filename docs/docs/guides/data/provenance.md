---
title: Trace a result with provenance
description: Find saved records, follow lineage, and query run evidence with read-only SQL.
---

# Trace a result with provenance

Start with a completed run. Use **Provenance** to follow the workflow, activities, and data behind its result.

## Investigate a result

1. Open the execution and copy its run ID and any produced data IDs.
2. Find the run in **Provenance → Explore**, then open its lineage.
3. If lineage does not answer the question, use read-only SQL to compare records or planned and observed values.
4. When sharing an investigation, export the relevant record or query page with its IDs, SQL, parameters, and page number.

For the API commands below, complete [API connection setup](/docs/tutorials/api-access) first.

## Explore provenance in Desktop

Open **Provenance**. The **Explore** tab loads a server-defined entity catalog. Select an entity, search across its safe projection, apply a field filter, sort a column, and page through the result. The current page can be exported as CSV or JSON.

<img src={require('@site/static/img/interface/provenance/explore-runs.png').default} alt="AkôFlow Desktop Provenance Explore view with the trusted-record catalog, Runs projection, search field, filter control, CSV and JSON exports, and lineage actions for each row." />

*Choose a record type first. In **Runs**, open a row for details or follow its lineage.*

The API exposes the same server-defined catalog and query:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/provenance/entities/"

curl --fail-with-body -G -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  --data-urlencode "q=completed" \
  --data-urlencode "filterField=status" \
  --data-urlencode "filterValue=completed" \
  --data-urlencode "page=1" \
  --data-urlencode "pageSize=50" \
  --data-urlencode "sortField=created_at" \
  --data-urlencode "sortOrder=desc" \
  "$AKOFLOW_API_URL/provenance/entities/runs/"
```

Entity names and fields are supplied by `/provenance/entities/`; clients should not invent them. Query responses include entity metadata, `items`, `page`, `pageSize`, `total`, and `hasNext`.

## Follow lineage

From an Explore result, choose **Open lineage**, or open the **Lineage** tab and provide an entity and ID. Select `upstream`, `downstream`, or `both`, choose a depth, then inspect nodes and relationships. Any node can become the new root.

<img src={require('@site/static/img/interface/provenance/lineage-fanout.png').default} alt="AkôFlow Desktop Lineage view for the completed SimGrid 30 GB fan-out run, showing record type and ID controls, direction and depth, the grouped lineage graph, graph filters, and the selected run details." />

*This completed run links to its plan, activity executions, transfers, workflow version, scope, and resources. Select a card to inspect its fields.*

### Read the Lineage screen

| Area | Use it for | Important interpretation |
| --- | --- | --- |
| **Record type** and **Record ID** | Define the root record. The current root can also come from **Open lineage** in Explore. | Use the stored ID, not a display name. IDs remain stable when a user changes a label. |
| **Direction** and **Depth** | Choose whether to follow antecedents, descendants, or both, then bound the search. | A larger depth adds relationships; it does not mean a later execution time. Start at 1 or 2 and expand only when the question requires it. |
| **Lineage graph** | Inspect the nodes grouped by graph distance from the root. | The heading reports the returned node and relationship counts. Grouped columns are distance from the root, not workflow stages or chronological lanes. |
| **Find a node** and **node-type filter** | Reduce a large graph to a specific record or entity kind such as transfers or activity executions. | Filtering changes the visible graph only. It does not change the lineage query or delete evidence. |
| **Selected-record panel** | Read the status and saved fields for the selected card, then use **Open record** for its page. | Compare the plan and run IDs when investigating planned versus observed behavior. |
| **Export JSON** | Preserve the exact lineage response for an investigation or a report. | The export is a snapshot of the current root, direction, and depth; record those choices with the file. |

For the API path, copy a run ID from the **Runs** results in Explore and enter it when prompted:

```bash
read -r -p 'Run ID: ' RUN_ID || exit 1
[ -n "$RUN_ID" ] || exit 1

curl --fail-with-body -G -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  --data-urlencode "direction=both" \
  --data-urlencode "depth=2" \
  --data-urlencode "maxNodes=300" \
  "$AKOFLOW_API_URL/provenance/lineage/runs/$RUN_ID/"
```

The response contains a `root` key, `nodes`, directed `edges`, and `truncated`. Start with a small depth: the graph may expand quickly, and the interface caps a request at 300 nodes.

## Query with read-only SQL

The **SQL** tab presents the queryable schema, templates for common investigations, named JSON parameters, result paging, explain, favorites, and local query history.

<img src={require('@site/static/img/interface/provenance/sql-planned-versus-observed.png').default} alt="AkôFlow Desktop Provenance SQL view showing the safe schema, a read-only query that compares planned and observed run durations, query controls, and the result summary." />

*The query compares completed runs with their plans. The displayed values come from the connected local database.*

### Read the SQL screen

| Area | Use it for | Important interpretation |
| --- | --- | --- |
| **Safe schema** | Discover the tables and columns that the service makes available to read-only queries. Click a field to insert its name into the editor. | This is the current service schema, not a generic SQLite browser. Start here instead of assuming a column exists. |
| **Query template** | Start a common investigation, then refine it in the editor. | A template is ordinary editable SQL. Review joins, filters, and ordering before relying on its output. |
| **Query editor** | Write a `SELECT` or `WITH` query, including named parameters. | The interface shows the active timeout and row limit. Statements that modify data are rejected. |
| **Run query** and **Query result** | Execute the query and inspect typed columns, rows, page controls, and elapsed milliseconds. | Row limits bound one result page. A “more rows available” message means that the result is not the complete matching set yet. |
| **Explain** | Inspect SQLite's query plan before using a costly investigation repeatedly. | An explanation describes the database's access plan; it does not replace the normal query result or prove a result is scientifically meaningful. |
| **Export CSV** and **Export JSON** | Save the current result page for analysis or a report. | Record the SQL, parameters, page, and time of export with the file so another investigator can reproduce it. |
| **Favorite** and **History** | Reuse a query in the same Desktop browser profile. | They are local conveniences, not shared provenance records. |

Only read-only `SELECT` and `WITH` queries are accepted. The Desktop communicates the current service limits as a 10-second execution timeout and 200 rows per page. Fetch the runtime schema instead of assuming table or column names:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/provenance/sql/schema/"

curl --fail-with-body -X POST -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sql":"SELECT id, status, created_at FROM execution_runs WHERE status = :status ORDER BY created_at DESC",
    "parameters":{"status":"completed"},
    "page":1,
    "pageSize":200
  }' \
  "$AKOFLOW_API_URL/provenance/sql/"
```

Send the same payload to `/provenance/sql/explain/` to inspect the query plan without running the ordinary result path. SQL results contain typed `columns`, `items`, pagination information, a `truncated` flag, and elapsed milliseconds.

For recorded connection checks, resource discovery, or console actions around the same time, [inspect audit events](/docs/guides/data/audit-events). Provenance endpoints return `503 Service Unavailable` when the explorer is not configured, `400 Bad Request` for invalid entity, SQL, or lineage requests, and `500 Internal Server Error` if schema discovery fails.
