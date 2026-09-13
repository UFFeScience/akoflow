---
title: Follow operation notifications
description: Track recent Desktop operations and find their durable records.
---

# Follow operation notifications

Notifications help you return to an operation started in this Desktop profile. For a saved record from any profile, [search by name or ID](./find-records); for a durable operational timeline, [inspect audit events](../data/audit-events).

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

There is no `/notifications/` endpoint. Complete [API connection setup](../../tutorials/api-access), then query the records that own the operation and open a specific ID when you need its detail:

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

## When a notification is missing

Notifications only cover operations started and tracked by the current Desktop profile. Opening the app after an operation started elsewhere does not reconstruct its notification history. Open the owning list or detail page, or search its ID.
