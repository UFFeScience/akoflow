---
title: Use the interactive console
description: Run one-shot remote commands and open streamed terminal sessions on AkôFlow resources.
---

import {ConnectionPath, TerminalPanelGuide} from '@site/src/components/InfrastructureWalkthrough';

# Use the interactive console

Use the console to inspect a connected resource or run a short diagnostic command. Choose the action that fits the task:

- a **console command** runs one command, records stdout, stderr and exit status, and returns a durable command record;
- an **interactive session** opens a remote terminal for a longer conversation.

Both require a resource configured for interactive access and a working connection. AkôFlow records these operations in the audit trail.

<ConnectionPath />

## Open a terminal in Desktop

1. Open **Infrastructure → Resources** and select a login node, partition, compute node, or another SSH-capable resource.
2. Select the terminal action on the resource detail page.
3. AkôFlow creates the session, returns to the resource, and opens the fixed terminal panel at the bottom.
4. Use session tabs to switch between active terminals.
5. Use **Export log** to download the captured text or **Close session** to release the remote terminal.

<TerminalPanelGuide />

Switching tabs keeps the remote session open. Use **Close session** when you finish. If the active stream disappears unexpectedly, Desktop requests closure; check the session list before opening a replacement.

### When the terminal action is unavailable

The action appears only for a resource configured for interactive access with a usable connection and credential. Check the resource and connection health first. For an HPC cluster, select the login node rather than the cluster or a batch-only partition. For a proxied site, use the connection with the proxy route.

## Open and manage a session through the API

Complete [API connection setup](../../tutorials/api-access) first.
Choose an interactive-capable resource in Desktop and use its saved ID below. Keep these commands in the same Bash session.

```bash
read -r -p 'Interactive resource ID: ' AKOFLOW_CONSOLE_RESOURCE_ID || exit 1
[ -n "$AKOFLOW_CONSOLE_RESOURCE_ID" ] || exit 1

jq -n --arg id "$AKOFLOW_CONSOLE_RESOURCE_ID" '{resourceId:$id}' | \
  curl --fail-with-body \
    -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/json' \
    --data-binary @- "$AKOFLOW_API_URL/console-sessions/" \
    -o console-session.json || exit 1

SESSION_ID=$(jq -er '.id' console-session.json) || exit 1
```

A successful creation returns a `connected` session with its resolved `runtimeId` and `connectionId`. The command saves that response in `console-session.json` and sets `SESSION_ID` for the following requests. `resourceId` is required. Creation returns `422` when resolution or terminal startup fails and `503` when interactive console support is unavailable. Session records can later be `closed` or `failed`.

List sessions while you work:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/console-sessions/"
```

### Stream protocol

Connect a WebSocket client to the daemon, replacing the port and session ID with your values (`SESSION_ID` holds the ID returned above):

```text
ws://127.0.0.1:<daemon-port>/akoflow-api/console-sessions/<session-id>/stream/
```

Use `wss` when the HTTP endpoint uses HTTPS. Ordinary text or binary WebSocket messages are terminal input. Resize messages are JSON:

```json
{"type":"resize","rows":40,"columns":120}
```

The server streams terminal output back as WebSocket messages. Because browser WebSocket APIs cannot attach an `Authorization` header, deployment authentication and origin policy for this route must be configured consistently with the Desktop runtime; do not expose the daemon publicly.

Download the archived session log:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/console-sessions/$SESSION_ID/log/" \
  --output "akoflow-$SESSION_ID.log"
```

The response is UTF-8 text. A missing session log returns `404`; unavailable console support returns `503`.

Close the session when finished:

```bash
curl --fail-with-body -X DELETE \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/console-sessions/$SESSION_ID/"
```

Closure succeeds with `204 No Content`; an unknown session returns `404`.

## Run a one-shot command

The current Desktop focuses on the interactive terminal. Use the same resource ID for a repeatable one-shot diagnostic through the API:

```bash
jq -n --arg id "$AKOFLOW_CONSOLE_RESOURCE_ID" '{
  resourceId:$id,
  command:"hostname && uname -a",
  workingDirectory:"/tmp",
  environment:{LC_ALL:"C"},
  cpuCores:1,
  memoryBytes:268435456,
  timeoutSeconds:30
}' | curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary @- "$AKOFLOW_API_URL/console-commands/"
```

`resourceId` and `command` are required. The default timeout is 30 seconds and the maximum is 3,600 seconds. The request waits for the runner and returns a `completed` or `failed` record with `stdout`, `stderr`, `exitCode`, `failure`, and provider `externalId` when available. Check the record's `status`; HTTP `201 Created` alone does not mean the command succeeded.

List recent commands:

```bash
curl --get --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  --data-urlencode 'limit=50' \
  "$AKOFLOW_API_URL/console-commands/"
```

Command creation returns `422` for an unknown or unbound resource, missing input, or an excessive timeout. A runner failure is recorded as `status: failed` in the `201 Created` response. The route returns `503` if console commands are unavailable.

## Access and safety

- Authorize the Engine-managed public key on every SSH hop before opening a session.
- Select a resource configured for interactive access with a working connection. An inventory record alone is not sufficient.
- Imported instance snapshots are read-only; opening, writing to, or closing a session is blocked with `423 Locked`.
- Terminal logs may contain command output and secrets printed by programs. Treat exported logs as sensitive operational data.
- Close sessions when finished; closing the detail page alone does not close a daemon-owned session.
