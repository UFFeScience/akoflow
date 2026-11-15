---
title: Publish execution results to AkôFlow Hub
---

Open **Settings → AkôFlow Hub** in Desktop and select **Sign in to AkôFlow Hub**.
Every project uses the same Hub service configured for the installation. Continue in the browser,
check the displayed code, sign in using your Hub session, and authorize one
Hub project. Desktop detects approval automatically. Your Hub password is never
entered in Desktop.

On a finished execution, select **Publish to Hub**. This action is also available
in the workflow's selected result inspector. Choose an existing publication or
explicitly create one. The new publication form generates an editable slug from
the title and submits to the daemon API. Publications default to private.
Select which optional sections to include: process logs, file references,
activity metrics, and execution events. All four start unchecked. The workflow
definition, scheduling context and execution summary are required. The daemon
removes unchecked sections before requesting a preview from the Hub, then
recomputes the bundle checksums. Review the workflow YAML,
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
browser storage. Disconnect the project to revoke access; grants
can also be revoked from the Hub account. Factory reset clears the saved
connection on the next daemon start. Read-only imported instances cannot connect
or publish. Instance archives do not carry Hub credentials.

Operators configure the single service origin through `AKOFLOW_HUB_URL`. Projects
cannot override it; sign-in enables the project connection without a separate
configuration step. If the configured service changes, credentials and pending
approvals for the previous service are cleared before any network request.
`AKOFLOW_HUB_BROWSER_URL` optionally routes authorization and publication links
through a browser-facing origin, such as `http://localhost:3100` for an SSH tunnel.
It points to the same service; server requests continue to use `AKOFLOW_HUB_URL`. HTTPS is required except for explicit local IP
addresses and localhost. Requests do not follow redirects. Protect the daemon API
with its existing operator authentication, and restrict access to trusted users.

For local development, `releases/dockerfiles/server-local.Dockerfile` replaces
only the daemon binary on top of the pinned v1.0.15 runtime. It runs connector,
HTTP security and architecture tests with the race detector before building.
The platform Compose can build that image while retaining the existing storage,
BuildKit service and rollback images. The local GCS emulator and Mailpit setup
is intended for development.

If the release registry cannot be reached, the local Dockerfile accepts
`AKOFLOW_RUNTIME_IMAGE` as a build argument. Preserve a separate tag for the
verified installed v1.0.15 runtime, then use that tag as the base. Do not replace
it with a newer release implicitly; the build replaces only the daemon binary.

## Embed a public workflow in the documentation

Each showcase uses the shared Hub widget to load its versioned DAG and execution summary directly from the public Hub. It also offers the exact import URI, YAML, native experiment capture, versioned source and validated output archive. Publishing a new version does not silently change an existing showcase's selected version.

The Hub serves the widget at `/embed/PUBLICATION_ID/VERSION`; append `?view=dag` for a DAG-focused view. For example:

```html
<iframe
  src="https://hub.akoflow.com/embed/0d02c2c1-da65-4a17-9919-a8cbcf703341/1"
  title="OpenOil ensemble: versioned DAG and execution results"
  width="100%"
  height="850"
  loading="lazy"
  sandbox="allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-downloads"
  allow="clipboard-write; fullscreen"
></iframe>
```

Embeds fetch public data without account credentials. Missing or private publications return 404. The production framing policy permits `akoflow.com`, `www.akoflow.com` and the Hub itself; other embedding origins require an explicit operator configuration change. The interactive graph describes the published workflow definition, while the summary and result links refer to recorded executions of that exact version.
