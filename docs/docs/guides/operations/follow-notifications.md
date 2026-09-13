---
title: Follow operation notifications
description: Track recent Desktop operations and find their durable records.
---

# Follow operation notifications

Notifications help you return to an operation started in this Desktop profile. For a saved record from any profile, [search by name or ID](./find-records). Open that record for its lasting status; [Audit](../data/audit-events) covers only some operation types.

## In Desktop

The bell in the top bar reports terminal states for operations started in this
Desktop profile:

- planning sessions;
- executions;
- artifact builds;
- interactive terminals;
- cloud provisioning;
- Desktop application updates.

Select an operation notification to mark it read and open its associated page. Use the check control to mark all current items read. The center retains at most 40 entries.

A notification appears when a tracked operation finishes or fails. Closed terminals also leave the active-session list.

:::note Profile-local state
Notification entries and the list of tracked operations live in browser local storage. They are not audit or provenance records, do not synchronize between Desktop profiles, and may disappear when site data is cleared. Use execution, planning, build, cloud-operation, or console details for lasting status. Audit records connection checks, resource discovery, and console actions.
:::

Native operating-system notifications are emitted only when the browser/renderer exposes the Notification API and permission has already been granted. The current interface does not prompt for notification permission.

## Find the operation through the API

There is no `/notifications/` endpoint. Complete
[API connection setup](../../tutorials/api-access), then query the record that
owns the operation:

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

# Cloud provisioning and other cloud operations
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/cloud-operations/"
```

For an artifact build, use the build ID shown in its detail page:

```bash
read -r -p 'Artifact build ID: ' AKOFLOW_BUILD_ID || exit 1
[ -n "$AKOFLOW_BUILD_ID" ] || exit 1
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/artifact-builds/$AKOFLOW_BUILD_ID/runs/"
```

Open a returned run or cloud-operation ID for its status and events. Desktop
application update notices are local to the app; there is no daemon
notification record to query for them. For connection checks, resource
discovery, or console actions, query `/audit-events/` with the relevant
session, connection, resource, or environment filter. It does not reconstruct
the notification history.

## When a notification is missing

Notifications only cover operations started and tracked by the current Desktop profile. Opening the app after an operation started elsewhere does not reconstruct its notification history. Open the owning list or detail page, or search its ID.
