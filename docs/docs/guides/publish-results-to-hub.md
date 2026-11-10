---
title: Publish execution results to AkôFlow Hub
---

Open **Settings → AkôFlow Hub** in Desktop. Enter the Hub origin, enable the
connection, and select **Sign in with AkôFlow Hub**. Continue in the browser,
check the displayed code, sign in using your Hub session, and authorize one
Hub project. Desktop detects approval automatically. Your Hub password is never
entered in Desktop.

On a finished execution, select **Publish to Hub**. This action is also available
in the workflow's selected result inspector. Choose an existing publication or
explicitly create one. Publications default to private. Review the workflow YAML,
logs, file references, capture counts and version changes before confirming.
Public destinations require an additional acknowledgement.

The daemon captures the selected execution and its persisted scheduling context,
using the historical workflow version rather than the current editable version.
The existing `akoflow.experiment` 1.0 protocol carries scheduling, activity and
file graph inputs, attempts, transfers, events, infrastructure observations,
metric summaries, and available process logs. File references are preserved;
datasets, complete runtime log archives and binaries are not copied. Common
credential fields and secret-bearing strings are redacted before review. Review
the content before publishing. Packages exceeding 8 MiB fail without truncating
observations or logs.

Confirmation uploads exactly the reviewed snapshot. Reviews expire after fifteen
minutes or when the account or destination visibility changes. Concurrent Hub
version changes require another review. Workflow versions and snapshots remain
immutable; the Hub stores metadata in PostgreSQL and content in GCS.

The daemon stores the project connection outside the exportable database, in
`credentials/hub/connection.json` beside the active instance database, with mode
`0600` inside a `0700` directory. Tokens never pass through renderer responses or
browser storage. Disconnect or disable the connection to revoke access; grants
can also be revoked from the Hub account. Factory reset clears the saved
connection on the next daemon start. Read-only imported instances cannot connect
or publish. Instance archives do not carry Hub credentials.

Operators can provide the initial origin through `AKOFLOW_HUB_URL`. This does not
enable publishing automatically. HTTPS is required except for explicit local IP
addresses and localhost. Requests do not follow redirects. Protect the daemon API
with its existing operator authentication, and restrict access to trusted users.

For local development, `releases/dockerfiles/server-local.Dockerfile` replaces
only the daemon binary on top of the pinned v1.0.15 runtime. It runs connector,
HTTP security and architecture tests with the race detector before building.
The platform Compose can build that image while retaining the existing storage,
BuildKit service and rollback images. The local GCS emulator and Mailpit setup
is intended for development.
