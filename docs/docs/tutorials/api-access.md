---
title: Set up API access for the tutorials
sidebar_label: API connection setup
description: Configure one API base URL and token convention for infrastructure tutorials.
---

Use this setup for the API path in the HPC and Google Cloud tutorials. You need
Bash, `curl`, `jq`, and an AkôFlow server whose address and credential you manage.
The [server installation guide](/docs/guides/operations/server-instance) explains
how to deploy one and choose its token. A development server is also suitable
when its connection settings are known.

Set the base URL **including** `/akoflow-api`, without a trailing slash:

```bash
set -o pipefail
export AKOFLOW_API_URL='http://127.0.0.1:8080/akoflow-api'
read -rsp 'Akoflow API token: ' AKOFLOW_API_TOKEN; printf '\n'
export AKOFLOW_API_TOKEN
```

`pipefail` keeps a failed HTTP request visible when its output is piped to
`jq`. Stop if a command fails before creating dependent records.

Replace the origin and port with your server's settings. Press Enter without a
token only for a local server configured without one. For packaged Desktop,
connection details are managed by its proxy; completing the graphical tutorials
does not require extracting its internal credential.

Check readiness and then access to a protected catalog:

```bash
curl --fail-with-body "$AKOFLOW_API_URL/preflight/" | jq
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environments/" | jq
```

The first response must report `server.available: true`; the second must return
a catalog, which may be empty. A `401` means the supplied credential was rejected.
A `403` may indicate that a server without a token rejects non-loopback access.

Use a fresh tutorial identity. The examples use `research-hpc` and `research-gcp`;
if those already exist, inspect them before continuing instead of resubmitting
a create request. Store only returned credential references in environment JSON.

Continue with [HPC registration](/docs/tutorials/register-hpc) or [Google Cloud connection](/docs/tutorials/connect-cloud).
