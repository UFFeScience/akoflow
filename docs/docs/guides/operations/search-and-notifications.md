---
title: Find records and follow notifications
description: Find AkôFlow records and follow operations in Desktop.
---

# Find records and follow notifications

Use search to open a workflow, run, environment, or other record by name or ID. Notifications point to operations that need attention. For a full history, open the record or its audit events.

## Search from Desktop

1. Focus **Search AkôFlow** in the top bar, press `/`, or press `⌘K`/`Ctrl+K`.
2. Enter an ID, name, state or other indexed value.
3. Select a result, or press Enter to open the first result.
4. Press Escape to close search.

With an empty query, search shows quick navigation destinations. As you type, it shows matching records; select one to open its detail or list page.

Search covers:

| Type | Representative indexed values |
|---|---|
| `workflow` | ID, external ID, name, namespace |
| `execution` | ID, title, resource/runtime IDs, failure and status |
| `artifact` | ID, name and version |
| `environment` | ID, name, description and status |
| `resource` | ID, name, provider, region, zone and type |
| `plan` | ID, algorithm, objective, workflow version and scope |
| `scope` | ID, name, topology and environment versions |
| `materialization` | ID, variant, digest, resource, run, activity, path and status |

Exact matches appear before partial matches.

## Search through the API

Complete [API connection setup](../../tutorials/api-access) first.

```bash
curl --get --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  --data-urlencode 'q=science' \
  --data-urlencode 'types=workflow,execution,artifact' \
  --data-urlencode 'limit=20' \
  "$AKOFLOW_API_URL/search/"
```

The response shape is:

```json
{
  "query": "science",
  "results": [
    {
      "type": "workflow",
      "id": "science-workflow",
      "title": "Science workflow",
      "subtitle": "default",
      "path": "/workflows/science-workflow",
      "score": 0.9
    }
  ],
  "total": 1
}
```

`limit` defaults to 30 and is capped at 100. Omit `types` to search every supported type. Unknown type names are ignored; if every supplied name is unknown, the result is empty. An empty `q` returns an empty result without loading catalogs. A catalog failure returns `500 Internal Server Error`.

## Follow operation notifications

The bell in the top bar reports completed or terminal states for operations started in this Desktop profile:

- planning sessions;
- executions;
- artifact builds;
- interactive terminals;
- cloud provisioning;
- Desktop application updates.

Select an operation notification to mark it read and open its associated page. Use the check control to mark all current items read. The center retains at most 40 entries.

A notification appears when a tracked operation finishes or fails. Closed terminals also leave the active-session list.

:::note Profile-local state
Notification entries and the list of tracked operations live in browser local storage. They are not audit or provenance records, do not synchronize between Desktop profiles, and may disappear when site data is cleared. Use **Audit**, execution details, planning details, or build details for durable operational evidence.
:::

Native operating-system notifications are emitted only when the browser/renderer exposes the Notification API and permission has already been granted. The current interface does not prompt for notification permission.

## API equivalents

There is no `/notifications/` endpoint. Query the records that own the operation, then open a specific ID from a result when you need its detail:

```bash
# Planning sessions
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/planning-sessions/"

# Execution runs
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/execution-runs/"

# Active interactive sessions
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/console-sessions/"
```

For cloud instances, enter the ID of a cloud environment shown in **Infrastructure → Environments**:

```bash
read -r -p 'Cloud environment ID: ' ENVIRONMENT_ID || exit 1
[ -n "$ENVIRONMENT_ID" ] || exit 1
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environments/$ENVIRONMENT_ID/cloud-instances/"
```

For a durable cross-domain timeline, query `/audit-events/` with the appropriate execution, session, connection, resource or environment filter.

## When results do not appear

- Wait until at least one non-whitespace character is entered; an empty server query deliberately returns no entities.
- Search only returns records visible through the same catalogs as their list pages.
- A red error message means the server request failed; check the server connection and retry.
- Notifications only cover operations started and tracked by the current Desktop profile. Opening the app after an operation was started elsewhere does not reconstruct a notification history.
