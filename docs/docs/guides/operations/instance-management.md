---
title: Manage an instance
description: Configure an AkôFlow instance, export and import sanitized snapshots, switch instances, and reset local state.
---

# Manage an instance

An AkôFlow **instance** contains your environments, workflows, plans, runs, and settings. Use this guide to inspect its identity, export a snapshot, open a read-only archive, or return to the writable instance. Export a snapshot before changing versions or resetting local state.

For direct API use, complete [API connection setup](../../tutorials/api-access) before running the commands below. To change theme or graph animation, use [Personal preferences](./personal-preferences).

## Inspect the active identity

### Using AkôFlow Desktop

Open **Settings → General**. The current interface exposes the workspace transfer relay setting; instance identity fields are read through the server but are not currently editable as a separate Desktop form.

The relay is an in-memory buffer per active transfer. It streams source output to destination input and does not persist the transferred payload. The default is 8 MiB; accepted values are 5–64 MiB.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/instance/"
```

To change the relay size, read the current instance, update that field, and send the complete object back:

```bash
set -o pipefail
curl --fail-with-body --silent \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/instance/" \
  | jq '.transferBufferBytes = 8388608' \
  | curl --fail-with-body \
      -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
      -H 'Content-Type: application/json' \
      -X PUT "$AKOFLOW_API_URL/instance/" \
      --data-binary @-
```

`id` and `name` are required. A zero buffer selects the 8 MiB default; values outside 5–64 MiB return `422 Unprocessable Entity`.

## Export a sanitized instance

### Using AkôFlow Desktop

1. Open **Settings → Data management**.
2. Optionally enable **Include artifact files**. Large artifact stores can produce a large ZIP.
3. Select **Export instance ZIP**.

The server creates a consistent database snapshot and removes tokens, private keys, credential references, and connection secrets. The ZIP manifest records that credentials were not included. Including artifacts adds artifact files but does not restore credentials.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/instances/default/export/?includeArtifacts=false" \
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
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/zip' \
  --data-binary @akoflow-instance.zip \
  "$AKOFLOW_API_URL/instances/import/" \
  -o imported-instance.json || exit 1

SNAPSHOT_ID=$(jq -er '.id' imported-instance.json) || exit 1

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/instances/"

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -X POST "$AKOFLOW_API_URL/instance-activations/$SNAPSHOT_ID/"
```

Import accepts at most 8 GiB compressed data, at most 100,000 archive entries, and at most 64 GiB expanded data. Symbolic links and unsafe or unsupported archives are rejected with `422`. Activation returns `202 Accepted` with `instance` and a `restarting` boolean.

## What read-only means

An active imported snapshot is for inspection. The Desktop displays an archive banner, hides or guards editing controls, and leaves read access available. At the HTTP layer, every non-`GET` request returns:

```json
{"error":"the selected instance is a read-only snapshot"}
```

with status `423 Locked`. The sole write exception is `POST /instance-activations/{instanceId}/`, which lets you return to `default` or select another snapshot. Credentials are redacted, so connection and execution actions cannot work from the imported copy.

## Factory reset

:::danger Permanent local deletion
Factory reset permanently removes the active AkôFlow catalog, environments, workflows, plans, runs, artifacts metadata, managed credentials and personal preferences. Export a snapshot first if any state must be retained. External SSH key files are retained only when they are outside the server-managed credential directory; the Desktop specifically notes that external SSH key files remain.
:::

### Using AkôFlow Desktop

1. Open **Settings → Danger zone**.
2. Read the deletion summary.
3. Confirm the reset using the control shown by the application.

### Using the API

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -X POST "$AKOFLOW_API_URL/factory-reset/"
```

Success is `204 No Content`. The endpoint returns `503` when reset support is unavailable and `422` when the reset operation fails. The server clears the database before removing managed Kubernetes token files. If that file cleanup fails, a `422` can arrive after the catalog has already been cleared; inspect the instance before retrying. Reset cannot run while a read-only snapshot is active because the read-only guard returns `423` first.
