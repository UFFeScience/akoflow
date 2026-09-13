---
title: Provenance and audit
description: Query local execution evidence safely with SQL and inspect the operational audit trail.
---

# Choose provenance or audit

AkôFlow keeps two complementary records:

- **Provenance** is a read-only SQL workspace over the local evidence database. Use it to compare workflows, plans, runs, activities, transfers, and data.
- **Audit** records operational actions such as discovery, connection use, console access, credentials, and workflow operations.

Use SQL when the question crosses several records. Use Audit to answer what operation happened, when, against which target, and with which outcome.

## Query local evidence

Open **Provenance** in Desktop. The page loads the service-provided safe schema and opens with a planned-versus-observed template. It accepts only `SELECT` and `WITH` statements, so it cannot change the local database.

<img src={require('@site/static/img/interface/provenance/sql-planned-versus-observed.png').default} alt="AkôFlow Desktop Provenance SQL view showing the safe schema, a read-only query that compares planned and observed run durations, query controls, and the result summary." />

*The query joins completed execution runs to their schedule plans. Values in the result are evidence from the connected local database, not fixed example values.*

### Start from a table

The **Safe schema** column is the quickest way to inspect a table. Click a table name and Desktop replaces the editor with a bounded query such as:

```sql
SELECT *
FROM "execution_runs"
LIMIT 100
```

Expand the table to see its available fields. Clicking a field appends its name to the editor, which is useful while refining a `SELECT`, join, or filter. Review the query and choose **Run query** when ready; selecting a table does not execute it automatically.

### Read the SQL screen

| Area | Use it for | Important interpretation |
| --- | --- | --- |
| **Safe schema** | Discover the tables and columns made available to read-only SQL. Click a table to create its starter query; click a field to insert its name. | This is the schema exposed by the connected service, not a generic SQLite browser. Start here rather than assuming a table or column exists. |
| **Query template** | Start a common investigation, then refine it in the editor. | A template is ordinary editable SQL. Review joins, filters, and ordering before relying on its output. |
| **Query editor** | Write a `SELECT` or `WITH` query, including named parameters. | The interface shows the active timeout and row limit. Statements that modify data are rejected. |
| **Run query** and **Query result** | Execute the query and inspect typed columns, rows, page controls, and elapsed milliseconds. | Row limits bound one result page. A “more rows available” message means the result is not the complete matching set yet. |
| **Explain** | Inspect SQLite's query plan before repeating a costly investigation. | An explanation describes the database access plan; it does not replace a normal result or prove a result is scientifically meaningful. |
| **Export CSV** and **Export JSON** | Save the current result page for analysis or a report. | Record the SQL, parameters, page, and time of export so another investigator can reproduce it. |
| **Favorite** and **History** | Reuse a query in the same Desktop browser profile. | They are local conveniences, not shared evidence records. |

Only read-only `SELECT` and `WITH` queries are accepted. Desktop uses a 10-second execution timeout and returns up to 200 rows per page. Fetch the runtime schema instead of assuming table or column names:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/akoflow-api/provenance/sql/schema/"

curl --fail-with-body -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sql":"SELECT id, status, created_at FROM execution_runs WHERE status = :status ORDER BY created_at DESC",
    "parameters":{"status":"completed"},
    "page":1,
    "pageSize":200
  }' \
  "$AKOFLOW_URL/akoflow-api/provenance/sql/"
```

Send the same payload to `/provenance/sql/explain/` to inspect the query plan. SQL results contain typed `columns`, `items`, pagination information, a `truncated` flag, and elapsed milliseconds.

:::note Local UI state
SQL favorites and recent-query history are stored in the browser profile. They are conveniences, not evidence records, and are not synchronized through the API.
:::

## Inspect the audit trail

Open **Audit** for a chronological record of infrastructure discovery, connections, console access, commands, credentials, and workflow activity. It starts in **All events** and groups the loaded records into discovery/resources, connections, console sessions, workflows, and credentials.

<img src={require('@site/static/img/interface/operations/audit-events.png').default} alt="AkôFlow Desktop Audit view showing the All events filter, chronological audit table, event targets, succeeded and failed outcomes, and operational summaries." />

*Each row keeps the event time, its machine-readable type, persisted target, outcome, and an operational summary. A colored outcome is a result to investigate, not a diagnosis by itself.*

| Area | Use it for | Important interpretation |
| --- | --- | --- |
| **All events** and category tabs | Narrow the visible list to discovery/resources, connections, console sessions, workflows, or credentials. | **All events** removes the local category filter; it does not request a different server-side result set. |
| **Time** | Correlate an operation with a run, connection check, or terminal session. | The value is displayed in the local browser time zone. Use IDs and API filters for exact cross-system correlation. |
| **Event** | Identify an operation class, such as `connection.health.checked`. | Event types are machine-readable, dot-separated names. The category tabs match their leading namespace. |
| **Target** | Locate the connection, environment, resource, session, execution, or system record affected by an event. | This is a persisted target identifier when available; it is not necessarily the friendly name shown elsewhere in Desktop. |
| **Outcome** | Distinguish `started`, `succeeded`, and `failed` records. | A failure means the recorded operation did not complete successfully. Read **Summary** before changing a configuration. |

The API supports server-side filtering:

```bash
curl --fail-with-body -G -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  --data-urlencode "environmentId=$ENVIRONMENT_ID" \
  --data-urlencode "outcome=failed" \
  --data-urlencode "limit=100" \
  "$AKOFLOW_URL/akoflow-api/audit-events/"
```

Available filter parameters are `eventType`, `environmentId`, `resourceId`, `connectionId`, `sessionId`, `executionId`, `outcome`, and `limit`.

## Investigation workflow

1. Open the execution and identify the run, activity, plan, and produced-data IDs relevant to the question.
2. Open **Provenance**, select the relevant table in **Safe schema**, and refine the generated `SELECT` with those IDs.
3. Use the planned-versus-observed or transfer template when the question spans plans and runs.
4. Open **Audit** to correlate infrastructure, connection, credential, or console operations around the same time.
5. Export the relevant SQL result and preserve the SQL, parameters, and IDs with the report.
