---
title: Instance management
description: Configure an AkôFlow instance, export and import sanitized snapshots, switch instances, and reset local state.
---

# Instance management

An AkôFlow **instance** is one control-plane installation and its catalog. Its identity contains an ID, name, optional description, organization and location, plus the transfer relay buffer. The Engine creates an identity automatically from the machine hostname during startup; the Desktop cannot proceed when `GET /instance/` is unavailable.

Set these variables for the API examples:

```bash
export AKOFLOW_URL='http://127.0.0.1:<daemon-port>/akoflow-api'
export AKOFLOW_TOKEN='<daemon-token>'
```

## Inspect the active identity

### Using AkôFlow Desktop

Open **Settings → General**. The current interface exposes the workspace transfer relay setting; instance identity fields are read through the Engine but are not currently editable as a separate Desktop form.

The relay is an in-memory buffer per active transfer. It streams source output to destination input and does not persist the transferred payload. The default is 8 MiB; accepted values are 5–64 MiB.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/instance/"
```

To change the relay size, first preserve the identity returned by `GET`, then send the complete object:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H 'Content-Type: application/json' \
  -X PUT "$AKOFLOW_URL/instance/" \
  -d '{
    "id":"akoflow-lab",
    "name":"AkôFlow lab",
    "description":"Research control plane",
    "organization":"Example Lab",
    "location":"Niterói",
    "transferBufferBytes":8388608
  }'
```

`id` and `name` are required. A zero buffer selects the 8 MiB default; values outside 5–64 MiB return `422 Unprocessable Entity`.

## Personal preferences

Theme and graph animation are associated with a stable browser-profile client ID, not with an authenticated user account. Desktop saves them in local storage immediately and attempts to synchronize them with the Engine. If the Engine is offline, local preferences keep the interface usable.

### Using AkôFlow Desktop

1. Open **Settings → General**.
2. Select **Light** or **Dark**.
3. Turn **Graph animation** on or off.

### Using the API

The client ID must contain 8–128 characters. The only accepted themes are `light` and `dark`.

```bash
CLIENT_ID='docs-client-01'

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H 'Content-Type: application/json' \
  -X PUT "$AKOFLOW_URL/user-preferences/$CLIENT_ID/" \
  -d '{"theme":"dark","animationsEnabled":false}'

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/user-preferences/$CLIENT_ID/"
```

## Export a sanitized instance

### Using AkôFlow Desktop

1. Open **Settings → Data management**.
2. Optionally enable **Include artifact files**. Large artifact stores can produce a large ZIP.
3. Select **Export instance ZIP**.

The Engine uses SQLite `VACUUM INTO` to create a consistent database snapshot. Tokens, private keys, credential references and connection secrets are redacted. The ZIP manifest records that credentials were not included. Including artifacts adds artifact files but does not restore credentials.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/instances/default/export/?includeArtifacts=false" \
  --output akoflow-instance.zip
```

The response is `application/zip`, not JSON. A failure after streaming begins may appear as a truncated invalid ZIP, so validate the resulting archive before retaining it as a backup.

## Import and open a read-only snapshot

Import does not replace the active database. It validates the ZIP format, manifest, redaction marker, database checksum and database schema, then stores a separate read-only snapshot.

### Using AkôFlow Desktop

1. In **Settings → Data management**, select **Choose instance ZIP**.
2. Choose an AkôFlow ZIP. The new card appears under **Available instances**.
3. Select **Open read-only snapshot** and confirm the daemon restart.
4. To leave it, return to the same section and select **Return to writable instance**.

The Desktop waits up to 90 seconds for the daemon after switching. When server-side restart is unavailable it asks you to restart the daemon yourself.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H 'Content-Type: application/zip' \
  --data-binary @akoflow-instance.zip \
  "$AKOFLOW_URL/instances/import/"

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/instances/"

SNAPSHOT_ID='<id returned by import>'
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -X POST "$AKOFLOW_URL/instance-activations/$SNAPSHOT_ID/"
```

Import accepts at most 8 GiB compressed data, at most 10,000 archive entries, and at most 64 GiB expanded data. Symbolic links and unsafe or unsupported archives are rejected with `422`. Activation returns `202 Accepted` with `instance` and a `restarting` boolean.

## What read-only means

An active imported snapshot is for inspection. The Desktop displays an archive banner, hides or guards editing controls, and leaves read access available. At the HTTP layer, every non-`GET` request returns:

```json
{"error":"the selected instance is a read-only snapshot"}
```

with status `423 Locked`. The sole write exception is `POST /instance-activations/{instanceId}/`, which lets you return to `default` or select another snapshot. Credentials are redacted, so connection and execution actions cannot work from the imported copy.

## Factory reset

:::danger Permanent local deletion
Factory reset permanently removes the active AkôFlow catalog, environments, workflows, plans, runs, artifacts metadata, managed credentials and personal preferences. Export a snapshot first if any state must be retained. External SSH key files are retained only when they are outside the Engine-managed credential directory; the Desktop specifically notes that external SSH key files remain.
:::

### Using AkôFlow Desktop

1. Open **Settings → Danger zone**.
2. Read the deletion summary.
3. Confirm the reset using the control shown by the application.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -X POST "$AKOFLOW_URL/factory-reset/"
```

Success is `204 No Content`. The endpoint returns `503` when reset support is unavailable and `422` when the reset operation fails. It cannot run while a read-only snapshot is active because the read-only guard returns `423` first.
