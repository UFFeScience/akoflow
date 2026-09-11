---
title: Interactive console and commands
description: Run one-shot remote commands and open streamed terminal sessions on AkôFlow resources.
---

import {ConnectionPath, TerminalPanelGuide} from '@site/src/components/InfrastructureWalkthrough';

# Interactive console and commands

AkôFlow exposes two related mechanisms:

- a **console command** runs one command, records stdout, stderr and exit status, and returns a durable command record;
- an **interactive session** opens a remote terminal owned by the Engine and streams terminal bytes over WebSocket.

Both resolve the selected resource to a runtime and connection. They are operational access paths and produce audit events.

<ConnectionPath />

## Open a terminal in Desktop

1. Open **Infrastructure → Resources** and select a login node, partition, compute node, or another SSH-capable resource.
2. Select the terminal action on the resource detail page.
3. AkôFlow creates the session, returns to the resource, and opens the fixed terminal panel at the bottom.
4. Use session tabs to switch between active terminals.
5. Use **Export log** to download the captured text or **Close session** to release the remote terminal.

<TerminalPanelGuide />

The panel polls active sessions every three seconds. Switching tabs closes only the local WebSocket for the previous view; it does not intentionally close that remote session. If the active stream disappears unexpectedly, Desktop requests session closure so the remote terminal is not left consuming resources.

### When the terminal action is unavailable

The action appears only after AkôFlow can resolve all three layers: a resource, a runtime binding that supports interactive execution, and a usable connection/credential. Check the resource health and binding first. For an HPC cluster, select the login node rather than an abstract cluster or a batch-only partition. For a proxied site, the daemon must use the connection that contains the proxy route.

## Open and manage a session through the API

```bash
export AKOFLOW_URL='http://127.0.0.1:<daemon-port>/akoflow-api'
export AKOFLOW_TOKEN='<daemon-token>'

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_URL/console-sessions/" \
  -d '{"resourceId":"hpc-login","actorId":"researcher@example.org"}'
```

The created session has `starting`, `connected`, `closed`, or `failed` status and returns the resolved `runtimeId` and `connectionId`. `resourceId` is required. Creation returns `422` when resolution or terminal startup fails and `503` when interactive console support is unavailable.

List and close sessions:

```bash
curl -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/console-sessions/"

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -X DELETE "$AKOFLOW_URL/console-sessions/$SESSION_ID/"
```

Closure succeeds with `204 No Content`; an unknown session returns `404`.

### Stream protocol

Connect a WebSocket client to:

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
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/console-sessions/$SESSION_ID/log/" \
  --output "akoflow-$SESSION_ID.log"
```

The response is UTF-8 text. A missing session log returns `404`; unavailable console support returns `503`.

## Run a one-shot command

The current Desktop focuses on the interactive terminal. Use the HTTP API for repeatable one-shot diagnostics:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_URL/console-commands/" \
  -d '{
    "resourceId":"hpc-login",
    "actorId":"researcher@example.org",
    "command":"hostname && uname -a",
    "workingDirectory":"/tmp",
    "environment":{"LC_ALL":"C"},
    "cpuCores":1,
    "memoryBytes":268435456,
    "timeoutSeconds":30
  }'
```

`resourceId` and `command` are required. The default timeout is 30 seconds and the maximum is 3,600 seconds. The returned record has `running`, `completed`, or `failed` status and may include `stdout`, `stderr`, `exitCode`, `failure` and the provider `externalId`.

List recent commands:

```bash
curl --get --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  --data-urlencode 'limit=50' \
  "$AKOFLOW_URL/console-commands/"
```

Command creation returns `422` for an unknown/unbound resource, invalid input, an excessive timeout, or runner failure. It returns `503` if console commands are unavailable.

## Access and safety

- Authorize the Engine-managed public key on every SSH hop before opening a session.
- Select a resource with a usable runtime binding and connection. A resource existing in inventory is not by itself sufficient.
- Imported instance snapshots are read-only; opening, writing to, or closing a session is blocked with `423 Locked`.
- Terminal logs may contain command output and secrets printed by programs. Treat exported logs as sensitive operational data.
- Close sessions when finished; closing the detail page alone does not close a daemon-owned session.
