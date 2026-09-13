---
title: Inspect audit events
description: Find operational events and correlate failures with the affected AkôFlow records.
---

# Inspect audit events

Use **Audit** to find what operation ran, when it ran, which record it targeted, and whether it succeeded. Audit covers connection checks, infrastructure discovery, console use, credentials, and workflow operations. For the scientific history of a result, [trace its provenance](./provenance).

## Investigate an operation

1. Find the operation in **Audit** and note its time, target ID, and outcome.
2. Open the target record or use the API filters to narrow events around that ID.
3. For a workflow result, [follow its provenance](./provenance) and compare the run ID and time with the audit event.

## Inspect the audit trail

Open **Audit**. It starts in **All events** and groups the loaded records into discovery/resources, connections, console sessions, workflows, and credentials.

<img src={require('@site/static/img/interface/operations/audit-events.png').default} alt="AkôFlow Desktop Audit view showing the All events filter, chronological audit table, event targets, succeeded and failed outcomes, and operational summaries." />

*Each row keeps the event time, its machine-readable type, the persisted target, the outcome, and an operational summary. In this capture, connection health checks show both a failed Kubernetes check and a successful cloud credential check; the colored outcome is a result to investigate, not a diagnosis by itself.*

### Read the Audit screen

| Area | Use it for | Important interpretation |
| --- | --- | --- |
| **All events** and category tabs | Narrow the visible list to discovery/resources, connections, console sessions, workflows, or credentials. | The Desktop fetches an audit list and applies these categories in the browser. **All events** removes that local category filter; it does not request a different server-side result set. |
| **Time** | Correlate an operation with a run, connection check, or terminal session. | The value is displayed in the local browser time zone. Use persisted IDs and API filters when an investigation needs exact cross-system correlation. |
| **Event** | Identify the operation class, such as `connection.health.checked`. | Event types are machine-readable, dot-separated names. The category tabs match their leading namespace. |
| **Target** | Locate the connection, environment, resource, session, execution, or system record affected by the event. | This is a persisted target identifier when one is available; it is not necessarily the friendly name shown elsewhere in Desktop. |
| **Outcome** | Quickly distinguish `started`, `succeeded`, and `failed` records. | A failure tells you that the recorded operation did not complete successfully. Read **Summary** and then inspect the target before changing a configuration. |
| **Summary** | Read the service-provided context or error associated with the event. | Treat it as operational evidence. It can include a runtime error returned by an external system, so do not copy it into public reports without reviewing it. |

For API queries, complete [API connection setup](../../tutorials/api-access) first. This request lists recent failures without requiring an environment ID:

```bash
curl --fail-with-body -G -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  --data-urlencode "outcome=failed" \
  --data-urlencode "limit=100" \
  "$AKOFLOW_API_URL/audit-events/"
```

Add `--data-urlencode "environmentId=<your-environment-id>"` when investigating one environment. Other filters are `eventType`, `resourceId`, `connectionId`, `sessionId`, `executionId`, and `limit`. Outcomes include `started`, `succeeded`, and `failed`. Desktop applies its category tabs to the list it has loaded; use API filters when you need a specific server query.
